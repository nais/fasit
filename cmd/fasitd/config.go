package main

import "time"

type Config struct {
	BindAddress  string
	LogLevel     string
	FasitAddress string
	TenantName   string
	Env          string
	IAPAudience  string
	Insecure     bool
	Production   bool
	DryRun       bool
	// TerminationGracePeriod must match the pod's terminationGracePeriodSeconds.
	TerminationGracePeriod time.Duration
}

func DefaultConfig() Config {
	return Config{
		BindAddress:  ":8080",
		LogLevel:     "info",
		FasitAddress: "localhost:4445",
		Insecure:     false,
		DryRun:       true,
	}
}
