package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	logcache "code.cloudfoundry.org/go-log-cache"
	"github.com/apoydence/cf-canary-router/internal/command"
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
	downloader := command.NewGithubReleaseDownloader("apoydence/cf-canary-router", httpClient, logger)

	switch args[0] {
	case "canary-router":
		api, err := conn.ApiEndpoint()
		if err != nil {
			logger.Fatalf("%s", err)
		}

		skipSSLValidation, err := conn.IsSSLDisabled()
		if err != nil {
			logger.Fatalf("%s", err)
		}

		c := logcache.NewClient(
			strings.Replace(api, "api", "log-cache", 1),
			logcache.WithHTTPClient(
				&tokenHTTPClient{
					getToken: conn.AccessToken,
					c: &http.Client{
						Timeout: 5 * time.Second,
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{
								InsecureSkipVerify: skipSSLValidation,
							},
						},
					},
				},
			),
		)

		command.PushCanaryRouter(conn, os.Stdin, args[1:], downloader, c.Read, logger)
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
						"path":                "Path to the canary-router app to push (defaults to downloading release from github)",
						"name":                "Name for the canary router (defaults to 'canary-router')",
						"username":            "Username to use when pushing the app (REQUIRED)",
						"password":            "Password to use when pushing the app (REQUIRED)",
						"force":               "Skip warning prompt (default is false)",
						"canary-app":          "The new app to start routing data to (REQUIRED)",
						"current-app":         "The existing app to start routing data from (REQUIRED)",
						"plan":                `The migration plan (defaults to '{"Plan":[{"Percentage":10,"Duration":300000000000}]}')`,
						"query":               "The PromQL query that determines if the canary is successful (REQUIRED)",
						"skip-ssl-validation": "Whether to ignore certificate errors (default is false)",
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

// HTTPClient is the client used for HTTP requests
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type tokenHTTPClient struct {
	c        HTTPClient
	getToken func() (string, error)
}

func (c *tokenHTTPClient) Do(req *http.Request) (*http.Response, error) {
	token, err := c.getToken()
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", token)

	return c.c.Do(req)
}
