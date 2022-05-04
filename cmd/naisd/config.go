package main

type Config struct {
	BindAddress     string
	DBConnectionDSN string
	LogLevel        string
	EnvProjectID    string
	Env             string
	Kind            string
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
