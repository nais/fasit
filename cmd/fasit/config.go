package main

type Config struct {
	BindAddress          string
	GRPCBindAddress      string
	DBConnectionDSN      string
	LogLevel             string
	GCPProjectID         string
	StatusSubscriptionID string

	InsecureSkipProxy      bool
	InsecureSkipTokenCheck bool
	IAPAudience            string
}

func DefaultConfig() Config {
	return Config{
		BindAddress:     ":8080",
		GRPCBindAddress: ":4444",
		LogLevel:        "info",
	}
}
