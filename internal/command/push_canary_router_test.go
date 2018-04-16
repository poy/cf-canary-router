package command_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"code.cloudfoundry.org/cli/plugin"
	"code.cloudfoundry.org/cli/plugin/models"
	"github.com/apoydence/cf-canary-deploy/internal/command"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TP struct {
	*testing.T

	logger     *stubLogger
	cli        *stubCliConnection
	downloader *stubDownloader
	reader     *strings.Reader
}

func TestPushCanaryRouter(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TP {
		cli := newStubCliConnection()
		cli.apiEndpoint = "https://api.something.com"

		downloader := newStubDownloader()
		downloader.path = "/downloaded/temp/dir/canary_router"

		return TP{
			T:          t,
			logger:     &stubLogger{},
			cli:        cli,
			reader:     strings.NewReader("y\n"),
			downloader: downloader,
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
				"--canary-route", "https://some.route",
				"--current-route", "https://some.other.route",
				"--query", "some-query",
				"--plan", `{"Plan":[{"Percentage":99,"Duration":1000}]}`,
				"--skip-ssl-validation",
			},
			t.downloader,
			t.logger,
		)

		Expect(t, t.logger.printMessages).To(Contain(
			"The canary router functionality is an experimental feature. " +
				"See https://github.com/apoydence/cf-canary-deploy for more details.\n" +
				"Do you wish to proceed? [y/N] ",
		))

		Expect(t, t.cli.cliCommandArgs).To(HaveLen(2))
		Expect(t, t.cli.cliCommandArgs[0]).To(Equal(
			[]string{
				"push", "canary-router",
				"-p", "some-temp-dir",
				"-b", "binary_buildpack",
				"-c", "./canary-router",
				"--health-check-type", "process",
				"--no-start",
			},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"set-env", "canary-router", "UAA_ADDR", "https://uaa.something.com"},
			[]string{"set-env", "canary-router", "LOG_CACHE_ADDR", "https://log-cache.something.com"},
			[]string{"set-env", "canary-router", "UAA_CLIENT", "cf"},
			[]string{"set-env", "canary-router", "UAA_USER", "some-user"},
			[]string{"set-env", "canary-router", "UAA_PASSWORD", "some-password"},
			[]string{"set-env", "canary-router", "CANARY_ROUTE", "https://some.route"},
			[]string{"set-env", "canary-router", "CANARY_ROUTE", "https://some.route"},
			[]string{"set-env", "canary-router", "CURRENT_ROUTE", "https://some.other.route"},
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
				"--canary-route", "https://some.route",
				"--current-route", "https://some.other.route",
				"--query", "some-query",
			},
			t.downloader,
			t.logger,
		)

		Expect(t, t.cli.cliCommandArgs).To(HaveLen(2))
		Expect(t, t.cli.cliCommandArgs[0]).To(Equal(
			[]string{
				"push", "canary-router",
				"-p", "/downloaded/temp/dir",
				"-b", "binary_buildpack",
				"-c", "./canary-router",
				"--health-check-type", "process",
				"--no-start",
			},
		))

		Expect(t, t.downloader.assetName).To(Equal("canary-router"))
		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"set-env", "canary-router", "UAA_ADDR", "https://uaa.something.com"},
			[]string{"set-env", "canary-router", "LOG_CACHE_ADDR", "https://log-cache.something.com"},
			[]string{"set-env", "canary-router", "UAA_CLIENT", "cf"},
			[]string{"set-env", "canary-router", "UAA_USER", "some-user"},
			[]string{"set-env", "canary-router", "UAA_PASSWORD", "some-password"},
			[]string{"set-env", "canary-router", "CANARY_ROUTE", "https://some.route"},
			[]string{"set-env", "canary-router", "CURRENT_ROUTE", "https://some.other.route"},
			[]string{"set-env", "canary-router", "QUERY", "some-query"},
			[]string{"set-env", "canary-router", "PLAN", `{"Plan":[{"Percentage":10,"Duration":300000000000}]}`},
			[]string{"set-env", "canary-router", "SKIP_SSL_VALIDATION", "false"},
		))

		Expect(t, t.cli.cliCommandArgs[1]).To(Equal(
			[]string{
				"start", "canary-router",
			},
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
				"--canary-route", "https://some.route",
				"--current-route", "https://some.other.route",
				"--query", "some-query",
			},
			t.downloader,
			t.logger,
		)

		Expect(t, t.logger.printMessages).To(Contain(
			"The canary router functionality is an experimental feature. " +
				"See https://github.com/apoydence/cf-canary-deploy for more details.\n" +
				"Do you wish to proceed? [y/N] ",
		))

		Expect(t, t.cli.cliCommandArgs).To(HaveLen(2))
		Expect(t, t.cli.cliCommandArgs[0]).To(Equal(
			[]string{
				"push", "some-name",
				"-p", "some-temp-dir",
				"-b", "binary_buildpack",
				"-c", "./canary-router",
				"--health-check-type", "process",
				"--no-start",
			},
		))

		Expect(t, t.cli.cliCommandWithoutTerminalOutputArgs).To(Contain(
			[]string{"set-env", "some-name", "UAA_ADDR", "https://uaa.something.com"},
			[]string{"set-env", "some-name", "LOG_CACHE_ADDR", "https://log-cache.something.com"},
			[]string{"set-env", "some-name", "UAA_CLIENT", "cf"},
			[]string{"set-env", "some-name", "UAA_USER", "some-user"},
			[]string{"set-env", "some-name", "UAA_PASSWORD", "some-password"},
			[]string{"set-env", "some-name", "CANARY_ROUTE", "https://some.route"},
			[]string{"set-env", "some-name", "CURRENT_ROUTE", "https://some.other.route"},
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
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
				},
				t.downloader,
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
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
				},
				t.downloader,
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
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
				},
				t.downloader,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("failed to push"))
	})

	o.Spec("fatally logs if the canary-router username is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--password", "some-password",
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
				},
				t.downloader,
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
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
				},
				t.downloader,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --password missing"))
	})

	o.Spec("fatally logs if the canary-route is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
				},
				t.downloader,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --canary-route missing"))
	})

	o.Spec("fatally logs if the current-route is not provided", func(t TP) {
		Expect(t, func() {
			command.PushCanaryRouter(
				t.cli,
				t.reader,
				[]string{
					"--path", "some-temp-dir",
					"--username", "some-user",
					"--password", "some-password",
					"--canary-route", "https://some.route",
					"--query", "some-query",
				},
				t.downloader,
				t.logger,
			)
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("required flag --current-route missing"))
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
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
				},
				t.downloader,
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
					"--canary-route", "https://some.route",
					"--current-route", "https://some.other.route",
					"--query", "some-query",
					"--plan", "invalid",
				},
				t.downloader,
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
						"--canary-route", "https://some.route",
						"--current-route", "https://some.other.route",
						"--query", "some-query",
						"--force",
					},
					t.downloader,
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

	getAppName  string
	getAppGuid  string
	getAppError error

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
	}
}

func (s *stubCliConnection) GetApp(name string) (plugin_models.GetAppModel, error) {
	s.getAppName = name
	return plugin_models.GetAppModel{
		Name: name,
		Guid: s.getAppGuid,
	}, s.getAppError
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
