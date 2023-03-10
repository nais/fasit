package database

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeaturesRepo interface {
	FeaturesForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]*model.Feature, error)
	FeatureByName(ctx context.Context, name string) (*model.Feature, error)
}

func (r *repo) FeatureByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := r.querier.FeatureByName(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("get feature by name from db: %w", err)
	}

	return &model.Feature{
		Name:    f.Name,
		Version: f.Version,
	}, nil
}

func (r *repo) FeaturesForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]*model.Feature, error) {
	features, err := r.querier.FeaturesForKind(ctx, kind.String())
	if err != nil {
		return nil, err
	}

	if !ci {
		return featuresFromSQL(features)
	}

	rollouts, err := r.querier.RolloutsForKind(ctx, kind.String())
	if err != nil {
		return nil, err
	}

	for _, ro := range rollouts {
		for i, f := range features {
			if f.Name == ro.Name {
				// delete feature from slice
				features = append(features[:i], features[i+1:]...)
				break
			}
		}
		features = append(features, gensql.FeaturesForKindRow{
			Name:          ro.Name,
			Description:   ro.Description,
			Version:       ro.Version,
			Chart:         ro.Chart,
			Source:        ro.Source,
			Timeout:       ro.Timeout,
			Dependencies:  ro.Dependencies,
			DefaultValues: ro.DefaultValues,
			Kinds:         ro.Kinds,
			Values:        ro.Values,
			Created:       ro.Created,
		})
	}

	// sort features by name
	sort.Slice(features, func(i, j int) bool {
		return features[i].Name < features[j].Name
	})

	return featuresFromSQL(features)
}

// func (r *repo) MissingDependencies(ctx context.Context, envID uuid.UUID, feature *model.Feature) ([]*model.Feature, error) {
// 	states, err := r.querier.FeatureStatesGet(ctx, envID)
// 	if err != nil {
// 		return nil, err
// 	}

// 	enabledFeatures := []string{}
// 	for _, state := range states {
// 		if state.Enabled {
// 			enabledFeatures = append(enabledFeatures, state.Name)
// 		}
// 	}

// 	f, err := r.querier.FeatureByName(ctx, feature.Name)
// 	if err != nil {
// 		return nil, fmt.Errorf("get feature by name from db: %w", err)
// 	}

// 	return feature.Dependencies.FindMissing(enabledFeatures), nil
// }

func featuresFromSQL(features []gensql.FeaturesForKindRow) ([]*model.Feature, error) {
	var ret []*model.Feature
	for _, f := range features {
		deps := model.Dependencies{}
		if err := json.Unmarshal(f.Dependencies.Bytes, &deps); err != nil {
			return nil, fmt.Errorf("unmarshal dependencies: %w", err)
		}

		valuesYAML := make(map[string]any)
		if err := json.Unmarshal(f.DefaultValues.Bytes, &valuesYAML); err != nil {
			return nil, fmt.Errorf("unmarshal default values: %w", err)
		}

		feature := &model.Feature{
			Name:        f.Name,
			Description: f.Description,
			Version:     f.Version,
			Chart:       f.Chart,
			Source:      f.Source,
			FeatureYAML: model.FeatureYAML{
				Dependencies: deps,
				Timeout:      time.Duration(f.Timeout.Int64) * time.Microsecond,
			},
			ValuesYAML: valuesYAML,
		}
		ret = append(ret, feature)
	}
	return ret, nil
}
