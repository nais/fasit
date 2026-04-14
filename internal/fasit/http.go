package fasit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/rollout"
	"github.com/nais/fasit/internal/server"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

func newHTTPServer(
	ctx context.Context,
	loadContext contextloader.LoaderFunc,
	cfg *Config,
	repo database.Repo,
	notifier *notifier.Notifier,
	publisher rollout.NewPublisher,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*http.Server, error) {
	resolver := graph.NewResolver(ctx, repo, notifier, publisher, log)
	graphServer, err := server.SetupGraph(loadContext, resolver, meter)
	if err != nil {
		return nil, fmt.Errorf("setting up graph: %w", err)
	}

	router, err := server.SetupRouter(ctx, loadContext, cfg.IAPAudience, cfg.InsecureSkipProxy, cfg.InsecureSkipTokenCheck, graphServer, repo, log)
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
