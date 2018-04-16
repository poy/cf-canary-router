package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"github.com/apoydence/cf-canary-deploy/internal/command"
)

type cli struct{}

func (c cli) Run(conn plugin.CliConnection, args []string) {
	if len(args) == 0 {
		log.Fatalf("Expected atleast 1 argument, but got 0.")
	}

	logger := newLogger(os.Stderr)
	httpClient := &http.Client{
		Timeout: 5 * time.Second,
	}
	downloader := command.NewGithubReleaseDownloader("apoydence/cf-canary-deploy", httpClient, logger)

	switch args[0] {
	case "canary-router":
		command.PushCanaryRouter(conn, os.Stdin, args[1:], downloader, logger)
	case "CLI-MESSAGE-UNINSTALL":
		return
	default:
		log.Fatalf("Unknown subcommand: %s", args[0])
	}
}

// version is set via ldflags at compile time.  It should be JSON encoded
// plugin.VersionType.  If it does not unmarshal, the plugin version will be
// left empty.
var version string

func (c cli) GetMetadata() plugin.PluginMetadata {
	var v plugin.VersionType
	// Ignore the error. If this doesn't unmarshal, then we want the default
	// VersionType.
	_ = json.Unmarshal([]byte(version), &v)

	return plugin.PluginMetadata{
		Name:    "canary-router",
		Version: v,
		Commands: []plugin.Command{
			{
				Name:     "canary-router",
				HelpText: "Pushes a canary router",
				UsageDetails: plugin.Usage{
					Usage: "canary-router",
					Options: map[string]string{
						"path":                "Path to the canary-router app to push",
						"name":                "Name for the canary router",
						"username":            "Username to use when pushing the app",
						"password":            "Password to use when pushing the app",
						"force":               "Skip warning prompt. Default is false",
						"canary-route":        "The new route to start routing data to",
						"current-route":       "The existing route to start routing data from",
						"plan":                "The migration plan",
						"query":               "The PromQL query that determines if the canary is successful",
						"skip-ssl-validation": "Whether to ignore certificate errors. Default is false",
					},
				},
			},
		},
	}
}

func main() {
	plugin.Start(cli{})
}

type logger struct {
	*log.Logger

	w io.Writer
}

func newLogger(w io.Writer) *logger {
	return &logger{
		Logger: log.New(os.Stdout, "", 0),
		w:      w,
	}
}

func (l *logger) Print(a ...interface{}) {
	fmt.Fprint(os.Stdout, a...)
}
