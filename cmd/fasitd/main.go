package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/nais/fasit/internal/fasitd/daemon"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"github.com/nais/fasit/internal/helm"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/oauth2"
	"google.golang.org/api/impersonate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/credentials/oauth"
	"k8s.io/client-go/rest"

	"cloud.google.com/go/compute/metadata"
)

// Version is set at build time via ldflags.
var Version = "dev"

var cfg = DefaultConfig()

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address for metrics")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "which log level to output")
	flag.StringVar(&cfg.FasitAddress, "fasit-address", cfg.FasitAddress, "Fasit fasitd gRPC address")
	flag.StringVar(&cfg.TenantName, "tenant-name", "test", "tenant name")
	flag.StringVar(&cfg.Env, "env", "dev", "cluster environment")
	flag.StringVar(&cfg.IAPAudience, "iap-audience", "", "IAP audience (OAuth client ID) for the ID token sent to Fasit")
	flag.BoolVar(&cfg.Insecure, "insecure", cfg.Insecure, "connect without TLS/IAP (local development)")
	flag.BoolVar(&cfg.Production, "production", false, "run in-cluster (real helm release inventory)")
}

func main() {
	flag.Parse()
	log := newLogger(cfg.LogLevel)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, log); err != nil {
		log.With("err", err).Error("fatal")
		os.Exit(1)
	}
}

func run(ctx context.Context, log *slog.Logger) error {
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		srv := &http.Server{Addr: cfg.BindAddress, Handler: mux, ReadHeaderTimeout: 2 * time.Second}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.With("err", err).Error("metrics server")
		}
	}()

	dialOpts, err := dialOptions(ctx)
	if err != nil {
		return err
	}

	conn, err := grpc.NewClient(cfg.FasitAddress, dialOpts...)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := protogen.NewFasitdClient(conn)

	opts := daemon.AgentOptions{
		Tenant:      cfg.TenantName,
		Environment: cfg.Env,
		Version:     Version,
		DryRun:      true,
	}

	if cfg.Production {
		kubeConfig, err := rest.InClusterConfig()
		if err != nil {
			return err
		}
		opts.ReleaseLister = helm.New(kubeConfig, "nais-system", log.With("subsystem", "helm"))
		opts.ReleaseInterval = 15 * time.Minute
	}

	agent := daemon.NewAgent(client, opts, log)

	for {
		if err := agent.Run(ctx); err != nil {
			log.With("err", err).Warn("fasitd session ended")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(3 * time.Second):
		}
	}
}

func dialOptions(ctx context.Context) ([]grpc.DialOption, error) {
	if cfg.Insecure {
		return []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}, nil
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(nil)),
	}
	if cfg.IAPAudience != "" {
		ts, err := iapTokenSource(ctx, cfg.IAPAudience)
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithPerRPCCredentials(oauth.TokenSource{TokenSource: ts}))
	}
	return opts, nil
}

// iapTokenSource mints the OIDC ID token used to authenticate through Google
// IAP. On GKE Workload Identity the metadata identity endpoint mints the token
// as the federated Kubernetes SA principal, which is not granted
// getOpenIdToken. Instead we impersonate the bound GSA: ADC first obtains a
// normal access token as that GSA, then calls IAM Credentials generateIdToken
// targeting the same GSA (self-impersonation), which only requires the GSA's
// serviceAccountTokenCreator grant on itself.
func iapTokenSource(ctx context.Context, audience string) (oauth2.TokenSource, error) {
	email, err := metadata.EmailWithContext(ctx, "default")
	if err != nil {
		return nil, fmt.Errorf("resolve workload identity service account: %w", err)
	}
	return impersonate.IDTokenSource(ctx, impersonate.IDTokenConfig{
		TargetPrincipal: email,
		Audience:        audience,
		IncludeEmail:    true,
	})
}

func newLogger(level string) *slog.Logger {
	lvl := new(slog.LevelVar)
	switch level {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "warn", "warning":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: lvl}))
}
