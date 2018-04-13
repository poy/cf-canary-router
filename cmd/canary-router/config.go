package main

import (
	"encoding/json"
	"log"

	"code.cloudfoundry.org/go-envstruct"
	"github.com/apoydence/cf-canary-deploy/internal/proxy"
)

type Config struct {
	Port         int    `env:"PORT, required"`
	CurrentRoute string `env:"CURRENT_ROUTE, required"`
	CanaryRoute  string `env:"CANARY_ROUTE, required"`
	LogCacheAddr string `env:"LOG_CACHE_ADDR, required"`

	UaaAddr         string `env:"UAA_ADDR, required"`
	UaaUser         string `env:"UAA_USER, required"`
	UaaPassword     string `env:"UAA_PASSWORD, required, noreport"`
	UaaClient       string `env:"UAA_CLIENT, required"`
	UaaClientSecret string `env:"UAA_CLIENT_SECRET, noreport"`

	Query string `env:"QUERY, required"`
	Plan  Plan   `env:"PLAN, required"`
}

func loadConfig() Config {
	cfg := Config{}
	if err := envstruct.Load(&cfg); err != nil {
		log.Fatal(err)
	}

	return cfg
}

type Plan struct {
	Plan proxy.Plan
}

func (p *Plan) UnmarshalEnv(data string) error {
	return json.Unmarshal([]byte(data), p)
}
