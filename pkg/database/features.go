package database

import (
	"context"
	"sort"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeaturesRepo interface {
	FeaturesForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]*model.Feature, error)
}

func (r *repo) featuresForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]gensql.Feature, error) {
	features, err := r.querier.FeaturesForKind(ctx, kind.String())
	if err != nil {
		return nil, err
	}
	if !ci {
		return features, nil
	}

	// fetch all rollouts for the given environment kind
	rollouts, err := r.querier.RolloutsForKind(ctx, kind.String())
	if err != nil {
		return nil, err
	}

	for _, ro := range rollouts {
		for i, f := range features {
			if f.Name == ro.FeatureName {
				// delete feature from slice
				features = append(features[:i], features[i+1:]...)
				break
			}
		}
		features = append(features, gensql.Feature{
			Name:    ro.FeatureName,
			Version: ro.Version,
			Created: ro.Created,
		})
	}

	// sort features by name
	sort.Slice(features, func(i, j int) bool {
		return features[i].Name < features[j].Name
	})

	return features, nil
}
