package fasit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/server"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

func newHTTPServer(ctx context.Context, loadContext contextloader.LoaderFunc, cfg *Config, meter metric.Meter, log logrus.FieldLogger) (*http.Server, error) {
	router, err := server.SetupRouter(ctx, loadContext, cfg.IAPAudience, cfg.InsecureSkipProxy, meter, log)
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
