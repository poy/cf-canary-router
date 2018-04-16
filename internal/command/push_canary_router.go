package command

import (
	"bufio"
	"encoding/json"
	"flag"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/apoydence/cf-canary-deploy/internal/proxy"
)

type Downloader interface {
	Download(assetName string) string
}

type Logger interface {
	Printf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
	Print(...interface{})
}

func PushCanaryRouter(cli plugin.CliConnection, reader io.Reader, args []string, d Downloader, log Logger) {
	f := flag.NewFlagSet("", flag.ContinueOnError)
	p := f.String("path", "", "")
	name := f.String("name", "canary-router", "")
	username := f.String("username", "", "")
	password := f.String("password", "", "")
	canaryRoute := f.String("canary-route", "", "")
	currentRoute := f.String("current-route", "", "")
	query := f.String("query", "", "")
	planStr := f.String("plan", "", "")
	force := f.Bool("force", false, "")
	skipSSLValidation := f.Bool("skip-ssl-validation", false, "")
	err := f.Parse(args)
	if err != nil {
		log.Fatalf("%s", err)
	}

	_ = planStr

	f.VisitAll(func(flag *flag.Flag) {
		if flag.Value.String() == "" && (flag.Name != "path" && flag.Name != "plan" && flag.Name != "skip-ssl-validation") {
			log.Fatalf("required flag --%s missing", flag.Name)
		}
	})

	plan := parsePlan(*planStr, log)

	if !*force {
		log.Print(
			"The canary router functionality is an experimental feature. ",
			"See https://github.com/apoydence/cf-canary-deploy for more details.\n",
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
		"--health-check-type", "process",
		"--no-start",
	)
	if err != nil {
		log.Fatalf("%s", err)
	}

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
		"CANARY_ROUTE":        *canaryRoute,
		"CURRENT_ROUTE":       *currentRoute,
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
