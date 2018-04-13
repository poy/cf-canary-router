package proxy_test

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/apoydence/cf-canary-deploy/internal/proxy"
	"github.com/apoydence/onpar"
	. "github.com/apoydence/onpar/expect"
	. "github.com/apoydence/onpar/matchers"
)

type TP struct {
	*testing.T
	p *proxy.Proxy

	oldSpyServer *spyServer
	newSpyServer *spyServer

	oldTestServer *httptest.Server
	newTestServer *httptest.Server

	spyPlanner *spyPlanner
}

func TestProxy(t *testing.T) {
	t.Parallel()
	o := onpar.New()
	defer o.Run(t)

	o.BeforeEach(func(t *testing.T) TP {
		oldSpyServer := newSpyServer()
		oldTestServer := httptest.NewServer(oldSpyServer)

		newSpyServer := newSpyServer()
		newTestServer := httptest.NewServer(newSpyServer)
		spyPlanner := newSpyPlanner()

		return TP{
			T:             t,
			oldSpyServer:  oldSpyServer,
			oldTestServer: oldTestServer,

			newSpyServer:  newSpyServer,
			newTestServer: newTestServer,

			spyPlanner: spyPlanner,

			p: proxy.New(
				oldTestServer.URL,
				newTestServer.URL,
				spyPlanner,
				log.New(ioutil.Discard, "", 0),
			),
		}
	})

	o.AfterEach(func(t TP) {
		t.oldTestServer.Close()
		t.newTestServer.Close()
	})

	o.Spec("it follows the plan", func(t TP) {
		t.spyPlanner.percentage = 5

		for i := 0; i < 100; i++ {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "http://some.url", nil)
			Expect(t, err).To(BeNil())

			t.p.ServeHTTP(recorder, req)
		}

		Expect(t, len(t.oldSpyServer.requests)).To(Equal(95))
		Expect(t, len(t.newSpyServer.requests)).To(Equal(5))

		t.spyPlanner.percentage = 10

		t.oldSpyServer.clear()
		t.newSpyServer.clear()

		for i := 0; i < 100; i++ {
			recorder := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "http://some.url", nil)
			Expect(t, err).To(BeNil())

			t.p.ServeHTTP(recorder, req)
		}

		Expect(t, len(t.oldSpyServer.requests)).To(Equal(90))
		Expect(t, len(t.newSpyServer.requests)).To(Equal(10))

		var r *http.Request
		Expect(t, t.oldSpyServer.requests).To(Chain(Receive(), Fetch(&r)))
		// Skip the scheme
		Expect(t, r.Host).To(Equal(t.oldTestServer.URL[7:]))
		Expect(t, t.newSpyServer.requests).To(Chain(Receive(), Fetch(&r)))
		// Skip the scheme
		Expect(t, r.Host).To(Equal(t.newTestServer.URL[7:]))
	})
	o.Spec("it survives the race detector", func(t TP) {
		var wg sync.WaitGroup
		defer wg.Wait()

		for i := 0; i < 99; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				recorder := httptest.NewRecorder()
				req, err := http.NewRequest("GET", "http://some.url", nil)
				Expect(t, err).To(BeNil())

				t.p.ServeHTTP(recorder, req)
			}()
		}
	})
}

type spyServer struct {
	requests chan *http.Request
	bodies   chan []byte
}

func newSpyServer() *spyServer {
	return &spyServer{
		requests: make(chan *http.Request, 100),
		bodies:   make(chan []byte, 100),
	}
}

func (s *spyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}

	s.bodies <- body
	s.requests <- r
}

func (s *spyServer) clear() {
	s.requests = make(chan *http.Request, 100)
	s.bodies = make(chan []byte, 100)
}

type spyPlanner struct {
	percentage int
}

func newSpyPlanner() *spyPlanner {
	return &spyPlanner{}
}

func (s *spyPlanner) CurrentPercentage() int {
	return s.percentage
}
