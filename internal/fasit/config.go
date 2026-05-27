package fasit

import (
	"context"
	"fmt"

	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	HTTPBindAddress           string `env:"HTTP_BIND_ADDRESS,default=:8080"`
	GRPCBindAddress           string `env:"GRPC_BIND_ADDRESS,default=:4444"`
	DBConnectionDSN           string `env:"FASIT_DBCONN_STRING,default=postgres://postgres:postgres@localhost:5432/fasit?sslmode=disable"`
	LogLevel                  string `env:"LOG_LEVEL,default=info"`
	GCPProjectID              string `env:"GCP_PROJECT_ID,default=nais-local-dev"`
	StatusSubscriptionID      string `env:"PUBSUB_STATUS_SUBSCRIPTION_ID,default=fasit-subscription"`
	InsecureSkipProxy         bool   `env:"INSECURE_SKIP_PROXY,default=false"`
	IAPAudience               string `env:"IAP_AUDIENCE"`
	SlackAPIToken             string `env:"SLACK_API_TOKEN"`
	SlackChannelFeatureAlerts string `env:"SLACK_CHANNEL_FEATURE_ALERTS"`
}

// newConfig creates a new configuration instance from environment variables.
func newConfig(ctx context.Context, lookuper envconfig.Lookuper) (*Config, error) {
	cfg := &Config{}
	err := envconfig.ProcessWith(ctx, &envconfig.Config{
		Target:   cfg,
		Lookuper: lookuper,
	})
	if err != nil {
		return nil, fmt.Errorf("error processing configuration: %w", err)
	}
	return cfg, nil
}
