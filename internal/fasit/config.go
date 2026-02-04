package fasit

import (
	"flag"
	"os"

	"github.com/joho/godotenv"
)

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

	SlackAPIToken              string
	SlackClusterUpgradeChannel string
	SlackChannelFeatureAlerts  string
}

var cfg = Config{
	BindAddress:     ":8080",
	GRPCBindAddress: ":4444",
	LogLevel:        "info",
}

func init() {
	_ = godotenv.Load()
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.GRPCBindAddress, "grpc-bind-address", cfg.GRPCBindAddress, "Bind address")
	flag.StringVar(&cfg.DBConnectionDSN, "db-connection-dsn", getEnv("FASIT_DBCONN_STRING", "postgres://postgres:postgres@localhost:5432/fasit?sslmode=disable"), "database connection DSN")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.GCPProjectID, "project-id", "nais-local-dev", "Google project ID")
	flag.StringVar(&cfg.StatusSubscriptionID, "status-subscription-id", "fasit-subscription", "Pub/sub subscription for status")
	flag.StringVar(&cfg.IAPAudience, "iap-audience", "", "IAP audience string")
	flag.BoolVar(&cfg.InsecureSkipProxy, "insecure-skip-proxy", false, "Insecure, but allows the server to not require iap")
	flag.BoolVar(&cfg.InsecureSkipTokenCheck, "insecure-skip-token-check", false, "Insecure, but allows the server ignore token check")
	flag.StringVar(&cfg.SlackClusterUpgradeChannel, "slack-cluster-upgrade-channel", os.Getenv("SLACK_CLUSTER_UPGRADE_CHANNEL"), "Slack channel to send message to")
	flag.StringVar(&cfg.SlackChannelFeatureAlerts, "slack-channel-feature-alerts", os.Getenv("SLACK_CHANNEL_FEATURE_ALERTS"), "Slack channel to send feature alerts to")
	flag.StringVar(&cfg.SlackAPIToken, "slack-api-token", os.Getenv("SLACK_API_TOKEN"), "Slack API token")
}
