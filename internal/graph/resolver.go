package graph

import (
	"context"
	"fmt"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/cluster"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Repo           database.Repo
	Log            logrus.FieldLogger
	ClusterManager cluster.ClusterManager

	logNotifier     *logNotifier
	diNotifier      *updateNotifier
	createPublisher workers.NewPublisher
}

func NewResolver(ctx context.Context, repo database.Repo, notifier *notifier.Notifier, naisdPublisher workers.NewPublisher, clusterManager cluster.ClusterManager, log logrus.FieldLogger) *Resolver {
	return &Resolver{
		Repo:            repo,
		Log:             log.WithField("subsystem", "graphql"),
		ClusterManager:  clusterManager,
		createPublisher: naisdPublisher,
		logNotifier:     newLogNotifier(ctx, notifier, repo),
		diNotifier:      newDeployInstructionsNotifier(ctx, notifier, repo),
	}
}

func (r *Resolver) missingDependencies(ctx context.Context, featureName string, envID uuid.UUID) ([]*model.Feature, error) {
	f, err := r.Repo.FeatureByNameForEnv(ctx, featureName, envID)
	if err != nil {
		return nil, err
	}

	states, err := r.Repo.FeatureStatesGet(ctx, envID)
	if err != nil {
		return nil, err
	}

	enabledFeatures := []string{}
	for _, s := range states {
		if s.Enabled {
			enabledFeatures = append(enabledFeatures, s.FeatureName)
		}
	}

	ret := []*model.Feature{}

	for _, missing := range f.Dependencies.FindMissing(enabledFeatures) {
		mf, err := r.Repo.FeatureByNameForEnv(ctx, missing, envID)
		if err != nil {
			graphql.AddErrorf(ctx, "getting feature by name: %v: %w", missing, err)
			continue
		}
		ret = append(ret, mf)
	}
	return ret, nil
}

func (r *Resolver) deleteHelmInstallation(ctx context.Context, env *model.Environment, name string) error {
	tenant, err := r.Repo.TenantGet(ctx, env.TenantID)
	if err != nil {
		return fmt.Errorf("getting tenant: %w", err)
	}

	pub := r.createPublisher(workers.NaisdTopicID(tenant.Name, env.Name), r.Log)

	return pub.Publish(ctx, message.DeployInstruction{
		Name:      name,
		Uninstall: true,
	})
}
