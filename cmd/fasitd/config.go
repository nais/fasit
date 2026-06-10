package main

type Config struct {
	BindAddress  string
	LogLevel     string
	FasitAddress string
	TenantName   string
	Env          string
	IAPAudience  string
	Insecure     bool
	Production   bool
}

func DefaultConfig() Config {
	return Config{
		BindAddress:  ":8080",
		LogLevel:     "info",
		FasitAddress: "localhost:4445",
		Insecure:     true,
	}
}
