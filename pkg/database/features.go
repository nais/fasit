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
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
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
			} else if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return nil, fmt.Errorf("get rollout by name from db: %w", err)
			}

		}

		return nil, fmt.Errorf("get feature by name from db: %w", err)
	}

	fyaml, defaultValues, err := makeFeatureYAML(f.Kinds, f.Dependencies, f.Values, f.DefaultValues, nil, f.Timeout)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
		Name:        f.Name,
		Description: f.Description,
		Version:     f.Version,
		Chart:       f.Chart,
		Source:      f.Source,
		FeatureYAML: fyaml,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
	}, nil
}

func (r *repo) Features(ctx context.Context) ([]*model.Feature, error) {
	features, err := r.querier.Features(ctx)
	if err != nil {
		return nil, fmt.Errorf("get features from db: %w", err)
	}

	var ret []*model.Feature
	for _, f := range features {
		fyaml, defaultValues, err := makeFeatureYAML(f.Kinds, f.Dependencies, f.Values, f.DefaultValues, nil, f.Timeout)
		if err != nil {
			return nil, fmt.Errorf("make feature yaml: %w", err)
		}

		feature := &model.Feature{
			Name:        f.Name,
			Description: f.Description,
			Version:     f.Version,
			Chart:       f.Chart,
			Source:      f.Source,
			FeatureYAML: fyaml,
			ValuesYAML:  defaultValues,
			SpecVersion: "v2",
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
			if f.Name == ro.Name {
				// delete feature from slice
				features = append(features[:i], features[i+1:]...)
				break
			}
		}
	}

	for _, ro := range rollouts {
		features = append(features, gensql.FeaturesForKindRow{
			Name:          ro.Name,
			Description:   ro.Description,
			Version:       ro.Version,
			Chart:         ro.Chart,
			Source:        ro.Source,
			Dependencies:  ro.Dependencies,
			DefaultValues: ro.DefaultValues,
			Kinds:         ro.Kinds,
			Values:        ro.Values,
			Created:       ro.Created,
			Timeout:       ro.Timeout,
		})
	}

	// sort features by name
	sort.Slice(features, func(i, j int) bool {
		return features[i].Name < features[j].Name
	})

	ret, err := featuresFromSQL(features)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (r *repo) FeatureByNameForEnv(ctx context.Context, name string, envID uuid.UUID) (*model.Feature, error) {
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

func featuresFromSQL(features []gensql.FeaturesForKindRow) ([]*model.Feature, error) {
	var ret []*model.Feature
	for _, f := range features {
		fyaml, defaultValues, err := makeFeatureYAML(f.Kinds, f.Dependencies, f.Values, f.DefaultValues, nil, f.Timeout)
		if err != nil {
			return nil, fmt.Errorf("make feature yaml: %w", err)
		}

		feature := &model.Feature{
			Name:        f.Name,
			Description: f.Description,
			Version:     f.Version,
			Chart:       f.Chart,
			Source:      f.Source,
			FeatureYAML: fyaml,
			ValuesYAML:  defaultValues,
			SpecVersion: "v2",
		}
		ret = append(ret, feature)
	}
	return ret, nil
}

func makeFeatureYAML(kinds []string, deps, values, defaultValues, rename []byte, timeout int64) (model.FeatureYAML, map[string]json.RawMessage, error) {
	ret := model.FeatureYAML{
		Timeout: time.Duration(timeout) * time.Millisecond,
	}
	if err := json.Unmarshal(deps, &ret.Dependencies); err != nil {
		return ret, nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	var retDefaultVals map[string]json.RawMessage
	if err := json.Unmarshal(defaultValues, &retDefaultVals); err != nil {
		return ret, nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	ret.EnvironmentKinds = make([]model.EnvironmentKind, len(kinds))
	for i, k := range kinds {
		ret.EnvironmentKinds[i] = model.EnvironmentKind(k)
	}

	if err := json.Unmarshal(values, &ret.Values); err != nil {
		return ret, nil, fmt.Errorf("unmarshal values: %w", err)
	}

	if len(rename) > 0 {
		if err := json.Unmarshal(rename, &ret.Rename); err != nil {
			return ret, nil, fmt.Errorf("unmarshal rename: %w", err)
		}
	}

	return ret, retDefaultVals, nil
}
