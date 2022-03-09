package main

type Config struct {
	BindAddress     string
	DBConnectionDSN string
	LogLevel        string
	EnvProjectID    string
	Env             string
	PartnerName     string
	NaisProjectID   string
}

func DefaultConfig() Config {
	return Config{
		BindAddress: ":8080",
		LogLevel:    "info",
	}
}
