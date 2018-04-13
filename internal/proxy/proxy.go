package proxy

import (
	"log"
	"math/rand"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync/atomic"
)

type Proxy struct {
	oldRp   *httputil.ReverseProxy
	newRp   *httputil.ReverseProxy
	planner Planner
	idx     int64
}

type Planner interface {
	CurrentPercentage() int
}

func New(
	oldRoute string,
	newRoute string,
	planner Planner,
	log *log.Logger,
) *Proxy {
	oldU, err := url.Parse(oldRoute)
	if err != nil {
		log.Fatalf("failed to parse URL (%s): %s", oldRoute, err)
	}

	newU, err := url.Parse(newRoute)
	if err != nil {
		log.Fatalf("failed to parse URL (%s): %s", newRoute, err)
	}

	return &Proxy{
		oldRp:   httputil.NewSingleHostReverseProxy(oldU),
		newRp:   httputil.NewSingleHostReverseProxy(newU),
		planner: planner,

		// Seed with a random values to ensure all the proxies don't blast the
		// new route at thte same(ish) time.
		idx: rand.Int63(),
	}
}

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	idx := atomic.AddInt64(&p.idx, 13)

	// Host has to be cleared for the go-router. The reverse proxy does not
	// mess with the request host.
	r.Host = ""

	// This will only return true for the percentage of the time.
	if int(idx%100) < p.planner.CurrentPercentage() {
		p.newRp.ServeHTTP(w, r)
		return
	}

	p.oldRp.ServeHTTP(w, r)
}
