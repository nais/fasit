package main

type Config struct {
	BindAddress          string
	DBConnectionDSN      string
	LogLevel             string
	GCPProjectID         string
	DeploySubscriptionID string
	Env                  string
	PartnerName          string
	StatusTopicRef       string
}

func DefaultConfig() Config {
	return Config{
		BindAddress: ":8080",
		LogLevel:    "info",
	}
}
