package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	logcache "code.cloudfoundry.org/go-log-cache"
	"github.com/apoydence/cf-canary-deploy/internal/predicate"
	"github.com/apoydence/cf-canary-deploy/internal/proxy"
	"github.com/bradylove/envstruct"
)

func main() {
	log.Println("Starting canary router...")
	defer log.Println("Closing canary router...")

	cfg := loadConfig()
	envstruct.WriteReport(&cfg)

	reader := logcache.NewClient(
		cfg.LogCacheAddr,
		logcache.WithHTTPClient(
			logcache.NewOauth2HTTPClient(
				cfg.UaaAddr,
				cfg.UaaClient,
				cfg.UaaClientSecret,
				logcache.WithUser(cfg.UaaUser, cfg.UaaPassword),
			),
		),
	)

	predicate := predicate.NewPromQL(
		cfg.Query,
		reader,
		time.Tick(time.Second),
		log.New(os.Stderr, "", log.LstdFlags),
	)

	planner := proxy.NewRoutePlanner(
		cfg.Plan.Plan,
		predicate.Predicate,
		log.New(os.Stderr, "", log.LstdFlags),
	)

	proxy := proxy.New(
		cfg.CurrentRoute,
		cfg.CanaryRoute,
		planner,
		log.New(os.Stderr, "", log.LstdFlags),
	)

	// Health endpoint
	log.Fatal(
		http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), proxy),
	)
}
