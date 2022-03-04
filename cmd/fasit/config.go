package main

type Config struct {
	BindAddress     string
	DBConnectionDSN string
	LogLevel        string
}

func DefaultConfig() Config {
	return Config{
		BindAddress: ":8080",
		LogLevel:    "info",
	}
}
