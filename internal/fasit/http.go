package fasit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/nais/fasit/internal/cluster"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/server"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

func newHttpServer(
	ctx context.Context,
	setupContext SetupContextFunc,
	cfg *Config,
	repo database.Repo,
	notifier *notifier.Notifier,
	publisher workers.NewPublisher,
	clusterClient *cluster.Client,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*http.Server, error) {
	resolver := graph.NewResolver(ctx, repo, notifier, publisher, clusterClient, log)
	graphServer, err := server.SetupGraph(setupContext, resolver, meter)
	if err != nil {
		return nil, fmt.Errorf("setting up graph: %w", err)
	}

	router, err := server.SetupRouter(ctx, setupContext, cfg.IAPAudience, cfg.InsecureSkipProxy, cfg.InsecureSkipTokenCheck, graphServer, repo, log)
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
