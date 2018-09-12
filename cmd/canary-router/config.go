package main

import (
	"encoding/json"
	"log"

	"code.cloudfoundry.org/go-envstruct"
	"github.com/apoydence/cf-canary-router/internal/proxy"
)

type Config struct {
	Port         int    `env:"PORT, required, report"`
	CurrentRoute string `env:"CURRENT_ROUTE, required, report"`
	CanaryRoute  string `env:"CANARY_ROUTE, required, report"`
	LogCacheAddr string `env:"LOG_CACHE_ADDR, required, report"`

	UaaAddr         string `env:"UAA_ADDR, required, report"`
	UaaUser         string `env:"UAA_USER, required, report"`
	UaaPassword     string `env:"UAA_PASSWORD, required"`
	UaaClient       string `env:"UAA_CLIENT, required, report"`
	UaaClientSecret string `env:"UAA_CLIENT_SECRET"`

	Query string `env:"QUERY, required, report"`
	Plan  Plan   `env:"PLAN, required, report"`

	SkipSSLValidation bool `env:"SKIP_SSL_VALIDATION"`
}

func loadConfig() Config {
	cfg := Config{}
	if err := envstruct.Load(&cfg); err != nil {
		log.Fatal(err)
	}

	envstruct.WriteReport(&cfg)

	return cfg
}

type Plan struct {
	Plan proxy.Plan
}

func (p *Plan) UnmarshalEnv(data string) error {
	return json.Unmarshal([]byte(data), p)
}
