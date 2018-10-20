package command

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	llog "log"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/poy/cf-canary-router/internal/proxy"
	"github.com/poy/cf-canary-router/internal/structuredlogs"
)

type Downloader interface {
	Download(assetName string) string
}

type Logger interface {
	Printf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Print(...interface{})
}

func PushCanaryRouter(
	cli plugin.CliConnection,
	reader io.Reader,
	args []string,
	d Downloader,
	r logcache.Reader,
	log Logger,
) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	p := f.String("path", "", "")
	name := f.String("name", "canary-router", "")
	username := f.String("username", "", "")
	password := f.String("password", "", "")
	canaryApp := f.String("canary-app", "", "")
	currentApp := f.String("current-app", "", "")
	query := f.String("query", "", "")
	planStr := f.String("plan", "", "")
	force := f.Bool("force", false, "")
	skipSSLValidation := f.Bool("skip-ssl-validation", false, "")
	err := f.Parse(args)
	if err != nil {
		log.Fatalf("%s", err)
	}

	f.VisitAll(func(flag *flag.Flag) {
		if flag.Value.String() == "" && (flag.Name != "path" && flag.Name != "plan" && flag.Name != "skip-ssl-validation") {
			log.Fatalf("required flag --%s missing", flag.Name)
		}
	})

	plan := parsePlan(*planStr, log)

	canaryM, err := cli.GetApp(*canaryApp)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if len(canaryM.Routes) == 0 {
		log.Fatalf("%s does not have a route", *canaryApp)
	}

	canaryR := canaryM.Routes[0]
	canaryRoute := fmt.Sprintf("https://%s.%s%s", canaryR.Host, canaryR.Domain.Name, canaryR.Path)

	currentM, err := cli.GetApp(*currentApp)
	if err != nil {
		log.Fatalf("%s", err)
	}

	if len(currentM.Routes) == 0 {
		log.Fatalf("%s does not have a route", *currentApp)
	}

	tempRoute := "canary-router-temp"
	currentR := currentM.Routes[0]
	currentRoute := fmt.Sprintf("https://%s.%s%s", tempRoute, currentR.Domain.Name, currentR.Path)

	if !*force {
		log.Print(
			"The canary router functionality is an experimental feature. ",
			"See https://github.com/poy/cf-canary-router for more details.\n",
			"Do you wish to proceed? [y/N] ",
		)

		buf := bufio.NewReader(reader)
		resp, err := buf.ReadString('\n')
		if err != nil {
			log.Fatalf("failed to read user input: %s", err)
		}
		if strings.TrimSpace(strings.ToLower(resp)) != "y" {
			log.Fatalf("OK, exiting.")
		}
	}

	if *p == "" {
		log.Printf("Downloading latest canary router from github...")
		*p = path.Dir(d.Download("canary-router"))
		log.Printf("Done downloading canary router from github.")
	}

	_, err = cli.CliCommand(
		"push", *name,
		"-p", *p,
		"-b", "binary_buildpack",
		"-c", "./canary-router",
		"--no-start",
		"--no-route",
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

	defer func() {
		cli.CliCommandWithoutTerminalOutput(
			"delete", *name, "-f",
		)
	}()

	// Map the canary app to the current route
	_, err = cli.CliCommandWithoutTerminalOutput(
		"map-route", *name,
		currentR.Domain.Name,
		"--hostname", currentR.Host,
		"--path", currentR.Path,
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

	// Map the current app to a temp route
	_, err = cli.CliCommandWithoutTerminalOutput(
		"map-route", *currentApp,
		currentR.Domain.Name,
		"--hostname", tempRoute,
		"--path", currentR.Path,
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

	defer func() {
		_, err = cli.CliCommandWithoutTerminalOutput(
			"unmap-route", *currentApp,
			currentR.Domain.Name,
			"--hostname", tempRoute,
			"--path", currentR.Path,
		)
		if err != nil {
			log.Fatalf("%s", err)
		}
	}()

	api, err := cli.ApiEndpoint()
	if err != nil {
		log.Fatalf("%s", err)
	}

	envs := map[string]string{
		"UAA_ADDR":            strings.Replace(api, "api", "uaa", 1),
		"LOG_CACHE_ADDR":      strings.Replace(api, "api", "log-cache", 1),
		"UAA_CLIENT":          "cf",
		"UAA_USER":            *username,
		"UAA_PASSWORD":        *password,
		"CANARY_ROUTE":        canaryRoute,
		"CURRENT_ROUTE":       currentRoute,
		"QUERY":               *query,
		"PLAN":                plan,
		"SKIP_SSL_VALIDATION": strconv.FormatBool(*skipSSLValidation),
	}

	for n, value := range envs {
		_, err := cli.CliCommandWithoutTerminalOutput(
			"set-env", *name, n, value,
		)
		if err != nil {
			log.Fatalf("%s", err)
		}
	}

	cli.CliCommand("start", *name)

	// Remove the route from the current app
	_, err = cli.CliCommandWithoutTerminalOutput(
		"unmap-route", *currentApp,
		currentR.Domain.Name,
		"--hostname", currentR.Host,
		"--path", currentR.Path,
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

	appInfo, err := cli.GetApp(*name)
	if err != nil {
		log.Fatalf("%s", err)
	}

	log.Printf(appInfo.Guid)

	envelopes := make(chan *loggregator_v2.Envelope, 10)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go logcache.Walk(
		ctx,
		appInfo.Guid,
		func(es []*loggregator_v2.Envelope) bool {
			for _, e := range es {
				envelopes <- e
			}

			return true
		},
		r,
		logcache.WithWalkBackoff(logcache.NewAlwaysRetryBackoff(time.Second)),
		logcache.WithWalkLogger(llog.New(os.Stderr, "", 0)),
	)

	s := structuredlogs.NewEventStream(func() string {
		for {
			e := <-envelopes
			if len(e.GetLog().GetPayload()) == 0 {
				continue
			}

			return string(e.GetLog().GetPayload())
		}
	}, nil)

	// Wait to see if the canary app succeeds
	log.Printf("Waiting for events")
	for {
		e := s.NextEvent()

		switch e.Code {
		case proxy.NextPlanStep:
			log.Printf(e.Message)
		case proxy.FinishedPlanSteps:
			log.Printf(e.Message)

			_, err = cli.CliCommandWithoutTerminalOutput(
				"map-route", *canaryApp,
				currentR.Domain.Name,
				"--hostname", currentR.Host,
				"--path", currentR.Path,
			)
			if err != nil {
				log.Fatalf("%s", err)
			}

			return
		case proxy.Abort:
			log.Printf(e.Message)

			_, err = cli.CliCommandWithoutTerminalOutput(
				"map-route", *currentApp,
				currentR.Domain.Name,
				"--hostname", currentR.Host,
				"--path", currentR.Path,
			)
			if err != nil {
				log.Fatalf("%s", err)
			}

			return
		}
	}
}

func parsePlan(planStr string, log Logger) string {
	type Plan struct {
		Plan proxy.Plan
	}

	if planStr == "" {
		p := Plan{
			Plan: proxy.Plan{
				{
					Percentage: 10,
					Duration:   5 * time.Minute,
				},
			},
		}

		s, _ := json.Marshal(p)
		return string(s)
	}

	var p Plan
	if err := json.Unmarshal([]byte(planStr), &p); err != nil {
		log.Fatalf("failed to parse plan: %s", err)
	}

	return planStr
}
