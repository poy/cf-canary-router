package command_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/cli/plugin/models"
	logcache "code.cloudfoundry.org/go-log-cache"
	"code.cloudfoundry.org/go-loggregator/rpc/loggregator_v2"
	"github.com/poy/cf-canary-router/internal/command"
	"github.com/poy/cf-canary-router/internal/proxy"
	"github.com/poy/cf-canary-router/internal/structuredlogs"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TP struct {
	*testing.T

	logger     *stubLogger
	cli        *stubCliConnection
	downloader *stubDownloader
	reader     *strings.Reader
	spyReader  *spyReader
}

func TestPushCanaryRouter(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TP {
		cli := newStubCliConnection()
		cli.apiEndpoint = "https://api.something.com"
		cli.getApp["current-app"] = plugin_models.GetAppModel{
			Routes: []plugin_models.GetApp_RouteSummary{{
				Host:   "current",
				Domain: plugin_models.GetApp_DomainFields{Name: "some.route"},
				Path:   "/v1",
			}},
		}

		cli.getApp["canary-app"] = plugin_models.GetAppModel{
			Routes: []plugin_models.GetApp_RouteSummary{{
				Host:   "canary",
				Domain: plugin_models.GetApp_DomainFields{Name: "some.route"},
				Path:   "/v1",
			}},
		}

		cli.getApp["canary-router"] = plugin_models.GetAppModel{
			Guid: "some-guid",
		}

		cli.getApp["some-name"] = plugin_models.GetAppModel{
			Guid: "some-guid",
		}

		downloader := newStubDownloader()
		downloader.path = "/downloaded/temp/dir/canary_router"

		spyReader := newSpyReader()
		event := structuredlogs.Event{Code: proxy.FinishedPlanSteps}
		writeEvent(event, spyReader)

		return TP{
			T:          t,
			logger:     &stubLogger{},
			cli:        cli,
			reader:     strings.NewReader("y\n"),
			downloader: downloader,
			spyReader:  spyReader,
		}
	})

	o.Spec("it pushes app from the given canary-router directory", func(t TP) {
		command.PushCanaryRouter(
			t.cli,
			t.reader,
			[]string{
				"--path", "some-temp-dir",
				"--username", "some-user",
				"--password", "some-password",
				"--canary-app", "canary-app",
				"--current-app", "current-app",
				"--query", "some-query",
				"--plan", `{"Plan":[{"Percentage":99,"Duration":1000}]}`,
				"--skip-ssl-validation",
			},
			t.downloader,
			t.spyReader.read,
			t.logger,
		)

		Expect(t, t.logger.printMessages).To(Contain(
			"The canary router functionality is an experimental feature. " +
				"See https://github.com/poy/cf-canary-router for more details.\n" +
				"Do you wish to proceed? [y/N] ",
		))

		Expect(t, t.cli.cliCommandArgs).To(HaveLen(2))
		Expect(t, t.cli.cliCommandArgs[0]).To(Equal(
			[]string{
				"push", "canary-router",
				"-p", "some-temp-dir",
				"-b", "binary_buildpack",
				"-c", "./canary-router",
				"--no-start",
				"--no-route",
			},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"set-env", "canary-router", "UAA_ADDR", "https://uaa.something.com"},
			[]string{"set-env", "canary-router", "LOG_CACHE_ADDR", "https://log-cache.something.com"},
			[]string{"set-env", "canary-router", "UAA_CLIENT", "cf"},
			[]string{"set-env", "canary-router", "UAA_USER", "some-user"},
			[]string{"set-env", "canary-router", "UAA_PASSWORD", "some-password"},
			[]string{"set-env", "canary-router", "CANARY_ROUTE", "https://canary.some.route/v1"},
			[]string{"set-env", "canary-router", "CURRENT_ROUTE", "https://canary-router-temp.some.route/v1"},
			[]string{"set-env", "canary-router", "QUERY", "some-query"},
			[]string{"set-env", "canary-router", "PLAN", `{"Plan":[{"Percentage":99,"Duration":1000}]}`},
			[]string{"set-env", "canary-router", "SKIP_SSL_VALIDATION", "true"},
		))

		Expect(t, t.cli.cliCommandArgs[1]).To(Equal(
			[]string{
				"start", "canary-router",
			},
		))
	})

	o.Spec("pushes downloaded app", func(t TP) {
		command.PushCanaryRouter(
			t.cli,
			t.reader,
			[]string{
				"--username", "some-user",
				"--password", "some-password",
				"--canary-app", "canary-app",
				"--current-app", "current-app",
				"--query", "some-query",
			},
			t.downloader,
			t.spyReader.read,
			t.logger,
		)

		Expect(t, t.cli.cliCommandArgs).To(HaveLen(2))
		Expect(t, t.cli.cliCommandArgs[0]).To(Equal(
			[]string{
				"push", "canary-router",
				"-p", "/downloaded/temp/dir",
				"-b", "binary_buildpack",
				"-c", "./canary-router",
				"--no-start",
				"--no-route",
			},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"map-route", "current-app", "some.route", "--hostname", "canary-router-temp", "--path", "/v1"},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"map-route", "canary-router", "some.route", "--hostname", "current", "--path", "/v1"},
		))

		Expect(t, t.downloader.assetName).To(Equal("canary-router"))
		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"set-env", "canary-router", "UAA_ADDR", "https://uaa.something.com"},
			[]string{"set-env", "canary-router", "LOG_CACHE_ADDR", "https://log-cache.something.com"},
			[]string{"set-env", "canary-router", "UAA_CLIENT", "cf"},
			[]string{"set-env", "canary-router", "UAA_USER", "some-user"},
			[]string{"set-env", "canary-router", "UAA_PASSWORD", "some-password"},
			[]string{"set-env", "canary-router", "CANARY_ROUTE", "https://canary.some.route/v1"},
			[]string{"set-env", "canary-router", "CURRENT_ROUTE", "https://canary-router-temp.some.route/v1"},
			[]string{"set-env", "canary-router", "QUERY", "some-query"},
			[]string{"set-env", "canary-router", "PLAN", `{"Plan":[{"Percentage":10,"Duration":300000000000}]}`},
			[]string{"set-env", "canary-router", "SKIP_SSL_VALIDATION", "false"},
		))

		Expect(t, t.cli.cliCommandArgs[1]).To(Equal(
			[]string{
				"start", "canary-router",
			},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"map-route", "current-app", "some.route", "--hostname", "canary-router-temp", "--path", "/v1"},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"unmap-route", "current-app", "some.route", "--hostname", "current", "--path", "/v1"},
		))
	})

	o.Spec("it pushes app with the given name", func(t TP) {
		command.PushCanaryRouter(
			t.cli,
			t.reader,
			[]string{
				"--path", "some-temp-dir",
				"--name", "some-name",
				"--username", "some-user",
				"--password", "some-password",
				"--canary-app", "canary-app",
				"--current-app", "current-app",
				"--query", "some-query",
			},
			t.downloader,
			t.spyReader.read,
			t.logger,
		)

		Expect(t, t.logger.printMessages).To(Contain(
			"The canary router functionality is an experimental feature. " +
				"See https://github.com/poy/cf-canary-router for more details.\n" +
				"Do you wish to proceed? [y/N] ",
		))

		Expect(t, t.cli.cliCommandArgs).To(HaveLen(2))
		Expect(t, t.cli.cliCommandArgs[0]).To(Equal(
			[]string{
				"push", "some-name",
				"-p", "some-temp-dir",
				"-b", "binary_buildpack",
				"-c", "./canary-router",
				"--no-start",
				"--no-route",
			},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"set-env", "some-name", "UAA_ADDR", "https://uaa.something.com"},
			[]string{"set-env", "some-name", "LOG_CACHE_ADDR", "https://log-cache.something.com"},
			[]string{"set-env", "some-name", "UAA_CLIENT", "cf"},
			[]string{"set-env", "some-name", "UAA_USER", "some-user"},
			[]string{"set-env", "some-name", "UAA_PASSWORD", "some-password"},
			[]string{"set-env", "some-name", "CANARY_ROUTE", "https://canary.some.route/v1"},
			[]string{"set-env", "some-name", "CURRENT_ROUTE", "https://canary-router-temp.some.route/v1"},
			[]string{"set-env", "some-name", "QUERY", "some-query"},
			[]string{"set-env", "some-name", "PLAN", `{"Plan":[{"Percentage":10,"Duration":300000000000}]}`},
			[]string{"set-env", "some-name", "SKIP_SSL_VALIDATION", "false"},
		))

		Expect(t, t.cli.cliCommandArgs[1]).To(Equal(
			[]string{
				"start", "some-name",
			},
		))
	})

	o.Spec("it sets the route to the current app if the canary aborts", func(t TP) {
		t.spyReader.envelopes = nil
		t.spyReader.errs = nil
		event := structuredlogs.Event{Code: proxy.Abort}
		writeEvent(event, t.spyReader)
		command.PushCanaryRouter(
			t.cli,
			t.reader,
			[]string{
				"--path", "some-temp-dir",
				"--name", "some-name",
				"--username", "some-user",
				"--password", "some-password",
				"--canary-app", "canary-app",
				"--current-app", "current-app",
				"--query", "some-query",
			},
			t.downloader,
			t.spyReader.read,
			t.logger,
		)

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"map-route", "current-app", "some.route", "--hostname", "current", "--path", "/v1"},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"unmap-route", "current-app", "some.route", "--hostname", "canary-router-temp", "--path", "/v1"},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"delete", "some-name", "-f"},
		))
	})

	o.Spec("it sets the route to the canary app if the canary succeeds", func(t TP) {
		t.spyReader.envelopes = nil
		t.spyReader.errs = nil
		event := structuredlogs.Event{Code: proxy.FinishedPlanSteps}
		writeEvent(event, t.spyReader)
		command.PushCanaryRouter(
			t.cli,
			t.reader,
			[]string{
				"--path", "some-temp-dir",
				"--username", "some-user",
				"--password", "some-password",
				"--canary-app", "canary-app",
				"--current-app", "current-app",
				"--query", "some-query",
			},
			t.downloader,
			t.spyReader.read,
			t.logger,
		)

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"map-route", "canary-app", "some.route", "--hostname", "current", "--path", "/v1"},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"unmap-route", "current-app", "some.route", "--hostname", "canary-router-temp", "--path", "/v1"},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"delete", "canary-router", "-f"},
		))
	})

	o.Spec("it fatally logs if confirmation is given anything other than y", func(t TP) {
		reader := strings.NewReader("no\n")

		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())

		Expect(t, t.logger.fatalfMessage).To(Equal("OK, exiting."))
	})

	o.Spec("fatally logs if fetching the api endpoint fails", func(t TP) {
		t.cli.apiEndpointError = errors.New("some-error")
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("some-error"))
	})

	o.Spec("fatally logs if the push fails", func(t TP) {
		t.cli.pushAppError = errors.New("failed to push")
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("failed to push"))
	})

	o.Spec("fatally logs if the GetApp for the canary-app fails", func(t TP) {
		delete(t.cli.getApp, "canary-app")
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("unknown app"))
	})

	o.Spec("fatally logs if the GetApp for the current-app fails", func(t TP) {
		delete(t.cli.getApp, "current-app")
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("unknown app"))
	})

	o.Spec("fatally logs if the GetApp for the canary-app doesn't have a route", func(t TP) {
		t.cli.getApp["canary-app"] = plugin_models.GetAppModel{}
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("canary-app does not have a route"))
	})

	o.Spec("fatally logs if the GetApp for the current-app doesn't have a route", func(t TP) {
		t.cli.getApp["current-app"] = plugin_models.GetAppModel{}
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("current-app does not have a route"))
	})

	o.Spec("fatally logs if the canary-router username is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --username missing"))
	})

	o.Spec("fatally logs if the canary-router password is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --password missing"))
	})

	o.Spec("fatally logs if the canary-app is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--current-app", "current-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --canary-app missing"))
	})

	o.Spec("fatally logs if the current-app is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--query", "some-query",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --current-app missing"))
	})

	o.Spec("fatally logs if the query is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --query missing"))
	})

	o.Spec("fatally logs if the plan does not parse", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-app", "canary-app",
					"--current-app", "current-app",
					"--query", "some-query",
					"--plan", "invalid",
				},
				t.downloader,
				t.spyReader.read,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("failed to parse plan: invalid character 'i' looking for beginning of value"))
	})

	o.Spec("fatally logs if setting env variables fails", func(t TP) {
		assert := func(env string) {
			t.cli.setEnvErrors[env] = errors.New("some-error")

			Expect(t, func() {
				command.PushCanaryRouter(
					t.cli,
					t.reader,
					[]string{
						"--path", "some-temp-dir",
						"--username", "some-user",
						"--password", "some-password",
						"--canary-app", "canary-app",
						"--current-app", "current-app",
						"--query", "some-query",
						"--force",
					},
					t.downloader,
					t.spyReader.read,
					t.logger,
				)
			}).To(Panic())
			Expect(t, t.logger.fatalfMessage).To(Equal("some-error"))
		}
		assert("UAA_ADDR")
		assert("LOG_CACHE_ADDR")
		assert("UAA_CLIENT")
		assert("UAA_USER")
		assert("UAA_PASSWORD")
		assert("CANARY_ROUTE")
		assert("CURRENT_ROUTE")
		assert("QUERY")
		assert("PLAN")
		assert("SKIP_SSL_VALIDATION")
	})
}

type stubLogger struct {
	fatalfMessage  string
	printfMessages []string
	printMessages  []string
}

func (l *stubLogger) Printf(format string, args ...interface{}) {
	l.printfMessages = append(l.printfMessages, fmt.Sprintf(format, args...))
}

func (l *stubLogger) Fatalf(format string, args ...interface{}) {
	l.fatalfMessage = fmt.Sprintf(format, args...)
	panic(l.fatalfMessage)
}

func (l *stubLogger) Print(a ...interface{}) {
	l.printMessages = append(l.printMessages, fmt.Sprint(a...))
}

type stubDownloader struct {
	path      string
	assetName string
}

func newStubDownloader() *stubDownloader {
	return &stubDownloader{}
}

func (s *stubDownloader) Download(assetName string) string {
	s.assetName = assetName
	return s.path
}

type stubCliConnection struct {
	plugin.CliConnection

	getApp map[string]plugin_models.GetAppModel

	cliCommandWithoutTerminalOutputArgs     [][]string
	cliCommandWithoutTerminalOutputResponse map[string]string

	cliCommandArgs [][]string
	pushAppError   error
	startAppError  error

	apiEndpoint      string
	apiEndpointError error

	setEnvErrors map[string]error
}

func newStubCliConnection() *stubCliConnection {
	return &stubCliConnection{
		cliCommandWithoutTerminalOutputResponse: make(map[string]string),
		setEnvErrors:                            make(map[string]error),
		getApp:                                  make(map[string]plugin_models.GetAppModel),
	}
}

func (s *stubCliConnection) GetApp(name string) (plugin_models.GetAppModel, error) {
	m, ok := s.getApp[name]
	if !ok {
		return plugin_models.GetAppModel{}, errors.New("unknown app")
	}

	return m, nil
}

func (s *stubCliConnection) CliCommandWithoutTerminalOutput(args ...string) ([]string, error) {
	s.cliCommandWithoutTerminalOutputArgs = append(
		s.cliCommandWithoutTerminalOutputArgs,
		args,
	)

	output, ok := s.cliCommandWithoutTerminalOutputResponse[strings.Join(args, " ")]
	if !ok {
		output = "{}"
	}

	var err error
	switch args[0] {
	case "set-env":
		err = s.setEnvErrors[args[2]]
	}

	return strings.Split(output, "\n"), err
}

func (s *stubCliConnection) CliCommand(args ...string) ([]string, error) {
	var err error
	switch args[0] {
	case "push":
		err = s.pushAppError
	case "start":
		err = s.startAppError
	}

	s.cliCommandArgs = append(s.cliCommandArgs, args)
	return nil, err
}

func (s *stubCliConnection) ApiEndpoint() (string, error) {
	return s.apiEndpoint, s.apiEndpointError
}

type spyEventStream struct {
	e structuredlogs.Event
}

func newSpyEventStream() *spyEventStream {
	return &spyEventStream{}
}

func (s *spyEventStream) NextEvent() structuredlogs.Event {
	return s.e
}

type spyReader struct {
	sourceIDs []string
	starts    []int64
	opts      [][]logcache.ReadOption

	envelopes [][]*loggregator_v2.Envelope
	errs      []error
}

func newSpyReader() *spyReader {
	return &spyReader{}
}

func (s *spyReader) read(ctx context.Context, sourceID string, start time.Time, opts ...logcache.ReadOption) ([]*loggregator_v2.Envelope, error) {
	s.sourceIDs = append(s.sourceIDs, sourceID)
	s.starts = append(s.starts, start.UnixNano())
	s.opts = append(s.opts, opts)

	if len(s.envelopes) != len(s.errs) {
		panic("envelopes and errs should have same len")
	}

	if len(s.envelopes) == 0 {
		return nil, nil
	}

	defer func() {
		s.envelopes = s.envelopes[1:]
		s.errs = s.errs[1:]
	}()

	return s.envelopes[0], s.errs[0]
}

func writeEvent(e structuredlogs.Event, r *spyReader) {
	eventData, _ := e.Marshal()
	r.envelopes = append(r.envelopes, []*loggregator_v2.Envelope{{
		Message: &loggregator_v2.Envelope_Log{
			Log: &loggregator_v2.Log{
				Payload: []byte(eventData),
			},
		},
	}})
	r.errs = append(r.errs, nil)
}
