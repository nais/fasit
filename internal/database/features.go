package database

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type FeaturesRepo interface {
	FeatureByName(ctx context.Context, name string) (*model.Feature, error)
	FeatureByNameForEnv(ctx context.Context, name string, envID uuid.UUID) (*model.Feature, error)
	FeatureVersionUpdate(ctx context.Context, name string, version string) error
	Features(ctx context.Context) ([]*model.Feature, error)
	FeaturesForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]*model.Feature, error)
}

func (r *repo) FeatureVersionUpdate(ctx context.Context, name string, version string) error {
	return r.querier.FeatureVersionUpdate(ctx, gensql.FeatureVersionUpdateParams{
		Name:    name,
		Version: version,
	})
}

func (r *repo) FeatureByName(ctx context.Context, name string) (*model.Feature, error) {
	f, err := r.querier.FeatureByName(ctx, name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r, err := r.RolloutByName(ctx, name)
			if err == nil {
				return r, nil
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("get rollout by name from db: %w", err)
			}

		}

		return nil, fmt.Errorf("get feature by name from db: %w", err)
	}

	return featureFromSQL(f.FeatureDatum)
}

func (r *repo) Features(ctx context.Context) ([]*model.Feature, error) {
	features, err := r.querier.Features(ctx)
	if err != nil {
		return nil, fmt.Errorf("get features from db: %w", err)
	}

	var ret []*model.Feature
	for _, f := range features {
		feature, err := featureFromSQL(f.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature: %w", err)
		}

		ret = append(ret, feature)
	}

	return ret, nil
}

func (r *repo) FeaturesForKind(ctx context.Context, kind model.EnvironmentKind, ci bool) ([]*model.Feature, error) {
	features, err := r.querier.FeaturesForKind(ctx, kind.String())
	if err != nil {
		return nil, err
	}

	if !ci {
		ret, err := featuresFromSQL(features)
		if err != nil {
			return nil, err
		}

		return ret, nil
	}

	rollouts, err := r.querier.RolloutsForKind(ctx, gensql.EnvironmentKind(kind))
	if err != nil {
		return nil, err
	}

	for _, ro := range rollouts {
		for i, f := range features {
			if f.FeatureDatum.Name == ro.FeatureDatum.Name {
				// delete feature from slice
				features = append(features[:i], features[i+1:]...)
				break
			}
		}
	}

	for _, ro := range rollouts {
		features = append(features, gensql.FeaturesForKindRow{
			FeatureDatum:   ro.FeatureDatum,
			Hasdeployments: ro.Hasdeployments,
		})
	}

	sort.Slice(features, func(i, j int) bool {
		return features[i].FeatureDatum.Name < features[j].FeatureDatum.Name
	})

	return featuresFromSQL(features)
}

func (r *repo) FeatureByNameForEnv(ctx context.Context, name string, envID uuid.UUID) (*model.Feature, error) {
	feat, err := r.V3GetEnvironmentFeature(ctx, envID, name)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("get environment feature from db: %w", err)
	}

	if feat != nil {
		return feat, nil
	}

	env, err := r.querier.EnvironmentGet(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("get environment from db: %w", err)
	}

	if env.Ci {
		roll, err := r.RolloutByName(ctx, name)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("get rollout by name from db: %w", err)
		}

		if roll != nil {
			return roll, nil
		}
	}

	return r.FeatureByName(ctx, name)
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

func featuresFromSQL(rows []gensql.FeaturesForKindRow) ([]*model.Feature, error) {
	ret := make([]*model.Feature, len(rows))
	for i, f := range rows {
		feature, err := featureFromSQL(f.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature: %w", err)
		}
		feature.HasDeployments = f.Hasdeployments
		ret[i] = feature
	}
	return ret, nil
}

func makeFeatureYAML(fd gensql.FeatureDatum) (model.FeatureYAML, map[string]json.RawMessage, error) {
	ret := model.FeatureYAML{
		Timeout: time.Duration(fd.Timeout) * time.Millisecond,
	}
	if err := json.Unmarshal(fd.Dependencies, &ret.Dependencies); err != nil {
		return ret, nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	var retDefaultVals map[string]json.RawMessage
	if err := json.Unmarshal(fd.DefaultValues, &retDefaultVals); err != nil {
		return ret, nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	ret.EnvironmentKinds = make([]model.EnvironmentKind, len(fd.Kinds))
	for i, k := range fd.Kinds {
		ret.EnvironmentKinds[i] = model.EnvironmentKind(k)
	}

	if err := json.Unmarshal(fd.Values, &ret.Values); err != nil {
		return ret, nil, fmt.Errorf("unmarshal values: %w", err)
	}

	if len(fd.Rename) > 0 {
		if err := json.Unmarshal(fd.Rename, &ret.Rename); err != nil {
			return ret, nil, fmt.Errorf("unmarshal rename: %w", err)
		}
	}

	return ret, retDefaultVals, nil
}
