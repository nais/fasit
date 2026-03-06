package graph

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/google/uuid"
	pgx "github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/deployment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// Configuration is the resolver for the configuration field.
func (r *configurationsResolver) Configuration(ctx context.Context, obj *model.Configurations) ([]*model.Configuration, error) {
	var configs []*model.Configuration
	var err error
	var feat *model.Feature

	var kind model.EnvironmentKind

	if obj.EnvID != nil && *obj.EnvID != uuid.Nil {
		feat, err = featurepkg.FeatureByNameForEnv(ctx, obj.FeatureName, *obj.EnvID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}

		configs, err = featurepkg.EnvConfig(ctx, feat, *obj.EnvID)
		if err != nil {
			return nil, err
		}

		env, err := r.Repo.EnvironmentGet(ctx, *obj.EnvID)
		if err != nil {
			return nil, err
		}
		kind = env.Kind
	} else {
		configs, err = featurepkg.ConfigGet(ctx, obj.FeatureName)
		if err != nil {
			return nil, err
		}

		if obj.RolloutID != uuid.Nil {
			feat, err = featurepkg.RolloutByName(ctx, obj.FeatureName)
			if err != nil {
				return nil, err
			}
		}

		if feat == nil {
			feat, err = featurepkg.FeatureByName(ctx, obj.FeatureName)
			if err != nil {
				return nil, err
			}
		}
	}

OUTER:
	for key, val := range feat.Values {
		val.GraphQLKey = key
		if val.Config == nil {
			continue
		}

		for _, c := range configs {
			if c.Key == key {
				c.Value = &val
				continue OUTER
			}
		}
		configs = append(configs, &model.Configuration{
			ID:      fakeUUID(feat.Name, key),
			Key:     key,
			Value:   &val,
			Content: feat.ValuesYAML[key],
			Source:  model.ConfigSourceHelm,
			GraphVars: struct {
				EnvironmentID *uuid.UUID
				FeatureName   string
			}{
				EnvironmentID: obj.EnvID,
				FeatureName:   obj.FeatureName,
			},
		})
	}

	for _, c := range configs {
		if c.Value == nil {
			c.Value = &model.Value{
				GraphQLKey: c.Key,
			}
			c.Source = model.ConfigSourceUnknown
		}
	}

	sourceWeight := func(c *model.Configuration) int {
		switch c.Source {
		case model.ConfigSourceUnknown:
			return -1
		case model.ConfigSourceEnv:
			return 0
		case model.ConfigSourceGlobal:
			return 1
		default:
			return 2
		}
	}

	sort.Slice(configs, func(i, j int) bool {
		tsi := sourceWeight(configs[i])
		tsj := sourceWeight(configs[j])
		if tsi == tsj {
			if configs[i].Value.Required && !configs[j].Value.Required {
				return true
			} else if !configs[i].Value.Required && configs[j].Value.Required {
				return false
			}

			return configs[i].Key < configs[j].Key
		}
		return tsi < tsj
	})

	configs = removeIgnoredKinds(configs, feat, kind)
	return configs, nil
}

// Computed is the resolver for the computed field.
func (r *configurationsResolver) Computed(ctx context.Context, obj *model.Configurations) ([]*model.ComputedValue, error) {
	if obj.EnvID == nil || *obj.EnvID == uuid.Nil {
		return nil, nil
	}
	f, err := featurepkg.FeatureByNameForEnv(ctx, obj.FeatureName, *obj.EnvID)
	if err != nil {
		return nil, fmt.Errorf("get feature by name for environment: %w", err)
	}
	cv, kind, err := featurepkg.MappingValuesForEnvironment(ctx, *obj.EnvID, false)
	if err != nil {
		return nil, err
	}

	computed := f.Values.Computed()

	generated := map[string]any{}
	if err := featurepkg.Generate(computed, kind, cv, generated); err != nil {
		return nil, fmt.Errorf("generate computed values: %w", err)
	}

	ret := []*model.ComputedValue{}
	for k, v := range computed {

		rm, err := json.Marshal(pluckFromMap(k, generated))
		if err != nil {
			return nil, fmt.Errorf("marshal computed value: %w", err)
		}

		v.GraphQLKey = k
		ret = append(ret, &model.ComputedValue{
			Value:   &v,
			Content: rm,
		})
	}

	return removeComputedIgnoredKinds(ret, f, kind), nil
}

// ConfigurationCreate is the resolver for the configurationCreate field.
func (r *mutationResolver) ConfigurationCreate(ctx context.Context, configuration model.NewConfiguration) (*model.Configuration, error) {
	var feature *model.Feature
	var err error

	if configuration.EnvironmentID != nil && *configuration.EnvironmentID != uuid.Nil {
		feature, err = featurepkg.FeatureByNameForEnv(ctx, configuration.Feature, *configuration.EnvironmentID)
	} else {
		feature, err = featurepkg.FeatureByName(ctx, configuration.Feature)
		if errors.Is(err, pgx.ErrNoRows) {
			feature, err = featurepkg.RolloutByName(ctx, configuration.Feature)
		}
	}
	if err != nil {
		return nil, err
	}

	val, ok := feature.Values[configuration.Key]
	if !ok {
		return nil, fmt.Errorf("key not found for feature %q", configuration.Feature)
	}
	if err := val.ValidConfig(configuration.Value); err != nil {
		return nil, fmt.Errorf("invalid configuration %q for %q: %w", configuration.Key, configuration.Feature, err)
	}

	configuration.Secret = val.Config.Secret
	ret, err := featurepkg.ConfigCreate(ctx, configuration)
	if err != nil {
		return nil, err
	}

	deployment.TriggerReconcile(ctx, deployment.ReconcileTriggerEvent{})

	return ret, nil
}

// ConfigurationUpdate is the resolver for the configurationUpdate field.
func (r *mutationResolver) ConfigurationUpdate(ctx context.Context, id uuid.UUID, configuration model.UpdateConfiguration) (*model.Configuration, error) {
	return featurepkg.ConfigUpdate(ctx, id, configuration)
}

// ConfigurationDelete is the resolver for the configurationDelete field.
func (r *mutationResolver) ConfigurationDelete(ctx context.Context, id uuid.UUID) (bool, error) {
	// TODO(thokra): Make this soft delete?
	if err := featurepkg.ConfigDelete(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// Configuration is the resolver for the configuration field.
func (r *queryResolver) Configuration(ctx context.Context, feature string, envID *uuid.UUID) (*model.Configurations, error) {
	return &model.Configurations{
		FeatureName: feature,
		EnvID:       envID,
	}, nil
}

// HelmValues is the resolver for the helmValues field.
func (r *queryResolver) HelmValues(ctx context.Context, feature string, envID *uuid.UUID, env *string, tenant *string) (json.RawMessage, error) {
	if envID == nil {
		if env == nil || tenant == nil {
			return nil, fmt.Errorf("environment id or name is required for helm values")
		}
		e, err := r.Repo.EnvironmentByNames(ctx, *tenant, *env)
		if err != nil {
			return nil, err
		}
		envID = &e.ID
	}
	f, err := featurepkg.FeatureByNameForEnv(ctx, feature, *envID)
	if err != nil {
		return nil, err
	}

	v, err := featurepkg.HelmValues(ctx, f, *envID)
	if err != nil {
		return nil, err
	}

	return json.Marshal(v)
}

func (r *Resolver) Configurations() graphgen.ConfigurationsResolver {
	return &configurationsResolver{r}
}

type configurationsResolver struct{ *Resolver }
