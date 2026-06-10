package fasit

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/api"
	"github.com/nais/fasit/internal/contextloader"
	"go.opentelemetry.io/otel/metric"
)

func newHTTPServer(ctx context.Context, loadContext contextloader.LoaderFunc, pool *pgxpool.Pool, cfg *Config, meter metric.Meter, log *slog.Logger) (*http.Server, error) {
	router, err := api.SetupRouter(ctx, loadContext, pool, cfg.IAPAudience, cfg.InsecureSkipProxy, meter, log, Version)
	if err != nil {
		return nil, fmt.Errorf("setting up router: %w", err)
	}

	return &http.Server{
		Addr:              cfg.HTTPBindAddress,
		Handler:           router,
		ReadHeaderTimeout: 2 * time.Second,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}, nil
}
