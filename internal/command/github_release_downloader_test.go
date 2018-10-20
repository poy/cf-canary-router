package command_test

import (
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/poy/cf-canary-router/internal/command"
	"github.com/poy/onpar"
	. "github.com/poy/onpar/expect"
	. "github.com/poy/onpar/matchers"
)

type TR struct {
	*testing.T
	httpClient *spyHTTPClient
	logger     *stubLogger
	d          command.GithubReleaseDownloader
}

func TestGithubReleaseDownloader(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TR {

		httpClient := newSpyHTTPClient()
		logger := &stubLogger{}
		return TR{
			T:          t,
			httpClient: httpClient,
			logger:     logger,
			d:          command.NewGithubReleaseDownloader("org-name/repo-name", httpClient, logger),
		}
	})

	o.Spec("returns a directory path to the latest release", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       releasesResponse(),
			},
		}

		t.httpClient.m["https://github.com/org-name/repo-name/releases/download/v0.5/space_drain"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(`Github File`))},
		}

		p := t.d.Download("space_drain")
		Expect(t, path.Base(p)).To(Equal("space_drain"))

		file, err := os.Open(p)
		Expect(t, err).To(Not(HaveOccurred()))

		contents, err := ioutil.ReadAll(file)
		Expect(t, err).To(Not(HaveOccurred()))

		Expect(t, string(contents)).To(Equal("Github File"))

		info, err := file.Stat()
		Expect(t, err).To(Not(HaveOccurred()))
		Expect(t, int(info.Mode()&0111)).To(Equal(0111))
	})

	o.Spec("works for any asset on the release", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       releasesResponse(),
			},
		}

		t.httpClient.m["https://github.com/org-name/repo-name/releases/download/v0.5/syslog_forwarder"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader(`Github File`)),
			},
		}

		p := t.d.Download("syslog_forwarder")
		Expect(t, path.Base(p)).To(Equal("syslog_forwarder"))
	})

	o.Spec("fatally logs when fetching releases returns a non-200", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			r: &http.Response{StatusCode: 404},
		}

		Expect(t, func() {
			t.d.Download("space_drain")
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("unexpected status code (404) from github"))
	})

	o.Spec("fatally logs when fetching the latest asset returns a non-200", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       releasesResponse(),
			},
		}

		t.httpClient.m["https://github.com/org-name/repo-name/releases/download/v0.5/space_drain"] = httpResponse{
			r: &http.Response{StatusCode: 404},
		}

		Expect(t, func() {
			t.d.Download("space_drain")
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("unexpected status code (404) from github"))
	})

	o.Spec("fatally logs when it can't find the space drain", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       releasesResponseNoSpaceDrain(),
			},
		}

		Expect(t, func() {
			t.d.Download("space_drain")
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("unable to find space_drain asset in releases"))
	})

	o.Spec("fatally logs when decoding releases fails", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			r: &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(strings.NewReader("invalid")),
			},
		}

		Expect(t, func() {
			t.d.Download("space_drain")
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("failed to decode releases response from github"))
	})

	o.Spec("fatally logs when github returns an error", func(t TR) {
		t.httpClient.m["https://api.github.com/repos/org-name/repo-name/releases"] = httpResponse{
			err: errors.New("some error"),
		}

		Expect(t, func() {
			t.d.Download("space_drain")
		}).To(Panic())
		Expect(t, t.logger.fatalfMessage).To(Equal("failed to read from github: some error"))
	})
}

func releasesResponse() io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(`
   [
     {
      "tag_name": "v0.4.1",
      "assets": [
        {
          "name": "something",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.4.1/something"
        },
        {
          "name": "space_drain",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.4.1/space_drain"
        },
        {
          "name": "syslog_forwarder",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.4.1/syslog_forwarder"
        }
      ]
     },
     {
      "tag_name": "v0.5",
      "assets": [
        {
          "name": "something",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.5/something"
        },
        {
          "name": "space_drain",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.5/space_drain"
        },
        {
          "name": "syslog_forwarder",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.5/syslog_forwarder"
        }
      ]
     }
   ]
`))
}

func releasesResponseNoSpaceDrain() io.ReadCloser {
	return ioutil.NopCloser(strings.NewReader(`
   [
     {
      "tag_name": "v0.5",
      "assets": [
        {
          "name": "something",
          "browser_download_url": "https://github.com/org-name/repo-name/releases/download/v0.5/something"
        }
      ]
     }
   ]
`))
}

type httpResponse struct {
	r   *http.Response
	err error
}

type spyHTTPClient struct {
	m map[string]httpResponse
}

func newSpyHTTPClient() *spyHTTPClient {
	return &spyHTTPClient{
		m: make(map[string]httpResponse),
	}
}

func (s *spyHTTPClient) Do(r *http.Request) (*http.Response, error) {
	if r.Method != http.MethodGet {
		panic("only use GETs")
	}

	value, ok := s.m[r.URL.String()]
	if !ok {
		panic("unknown URL " + r.URL.String())
	}

	return value.r, value.err
}
