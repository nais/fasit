package graph

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
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
}

func (r *Resolver) resolveFeatureByName(name string) (*model.Feature, error) {
	return r.Repo.FeatureByName(context.Background(), name)
}

func (r *Resolver) missingDependencies(ctx context.Context, featureName string, envID uuid.UUID) ([]*model.Feature, error) {
	f, err := r.Repo.FeatureByName(ctx, featureName)
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
		mf, err := r.Repo.FeatureByName(ctx, missing)
		if err != nil {
			return nil, fmt.Errorf("getting feature by name: %v: %w", missing, err)
		}
		ret = append(ret, mf)
	}
	return ret, nil
}
