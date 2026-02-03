package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type DeploymentRepo interface {
	GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error)
}

func (r *repo) GetEnvironmentFeature(ctx context.Context, environmentID uuid.UUID, featureName string) (*model.Feature, error) {
	f, err := r.querier.GetEnvironmentFeature(ctx, gensql.GetEnvironmentFeatureParams{
		EnvironmentID: environmentID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, err
	}

	feature, err := featureFromSQL(f.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("make feature: %w", err)
	}
	feature.HasDeployments = true

	return feature, nil
}

func featureFromSQL(f gensql.FeatureDatum) (*model.Feature, error) {
	fyaml, defaultValues, err := makeFeatureYAML(f)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
		FeatureYAML: fyaml,
		Name:        f.Name,
		Chart:       f.Chart,
		Version:     f.Version,
		Description: f.Description,
		Source:      f.Source,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
	}, nil
}
