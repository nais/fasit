package main

import "github.com/nais/fasit/pkg/graph/model"

type Config struct {
	BindAddress     string
	DBConnectionDSN string
	LogLevel        string
	EnvProjectID    string
	Env             string
	Kind            model.EnvironmentKind
	TenantName      string
	NaisProjectID   string
	Production      bool
}

func DefaultConfig() Config {
	return Config{
		BindAddress: ":8080",
		LogLevel:    "info",
	}
}
