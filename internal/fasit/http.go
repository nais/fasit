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
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/server"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

func newHttpServer(
	ctx context.Context,
	cfg *Config,
	repo database.Repo,
	deploymentManager *deployment.Manager,
	notifier *notifier.Notifier,
	publisher workers.NewPublisher,
	clusterClient *cluster.Client,
	meter metric.Meter,
	log logrus.FieldLogger,
) (*http.Server, error) {
	dependencies := &server.DomainHandlers{
		Repo:              repo,
		DeploymentManager: deploymentManager,
	}

	resolver := graph.NewResolver(ctx, repo, notifier, publisher, clusterClient, log)
	graphServer, err := server.SetupGraph(resolver, meter, dependencies)
	if err != nil {
		return nil, fmt.Errorf("setting up graph: %w", err)
	}

	router, err := server.SetupRouter(ctx, cfg.IAPAudience, cfg.InsecureSkipProxy, cfg.InsecureSkipTokenCheck, graphServer, dependencies, log)
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
