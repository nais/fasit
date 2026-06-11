package main

import (
	"fmt"
	"strings"
)

// route maps an inbound request host to the upstream apiserver host it should
// be proxied to. Each apiserver is exposed under its own predictable host so
// Fasit can store one issuer URL per environment.
type route struct {
	Host     string
	Upstream string
}

type Config struct {
	BindAddress string
	LogLevel    string
	Routes      []route
}

func DefaultConfig() Config {
	return Config{
		BindAddress: ":8080",
		LogLevel:    "info",
	}
}

// targets is a flag.Value accepting repeated host=upstream pairs.
type targets struct {
	routes *[]route
}

func (t targets) String() string {
	if t.routes == nil {
		return ""
	}
	parts := make([]string, 0, len(*t.routes))
	for _, r := range *t.routes {
		parts = append(parts, r.Host+"="+r.Upstream)
	}
	return strings.Join(parts, ",")
}

func (t targets) Set(v string) error {
	host, upstream, ok := strings.Cut(v, "=")
	if !ok || host == "" || upstream == "" {
		return fmt.Errorf("target must be host=upstream, got %q", v)
	}
	*t.routes = append(*t.routes, route{Host: host, Upstream: upstream})
	return nil
}
