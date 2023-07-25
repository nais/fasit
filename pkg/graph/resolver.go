package graph

import (
	"context"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/notifier"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Repo database.Repo
	Log  *logrus.Entry
	// HelmChartValues *helminfo.Cache

	notifier    *notifier.Notifier
	logNotifier *logNotifier
	diNotifier  *updateNotifier
}

func NewResolver(ctx context.Context, repo database.Repo, notifier *notifier.Notifier, log *logrus.Entry) *Resolver {
	return &Resolver{
		Repo:        repo,
		Log:         log,
		logNotifier: newLogNotifier(ctx, notifier, repo),
		diNotifier:  newDeployInstructionsNotifier(ctx, notifier, repo),
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
