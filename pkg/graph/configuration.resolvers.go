package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// Environment is the resolver for the environment field.
func (r *envConfigurationResolver) Environment(ctx context.Context, obj *model.EnvConfiguration) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

// Feature is the resolver for the feature field.
func (r *envConfigurationResolver) Feature(ctx context.Context, obj *model.EnvConfiguration) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// Feature is the resolver for the feature field.
func (r *globalConfigurationResolver) Feature(ctx context.Context, obj *model.GlobalConfiguration) (*model.Feature, error) {
	return r.resolveFeatureByName(obj.FeatureName)
}

// ConfigurationCreate is the resolver for the configurationCreate field.
func (r *mutationResolver) ConfigurationCreate(ctx context.Context, configuration model.NewConfiguration) (model.Configuration, error) {
	if err := r.Features.ValidConfig(configuration.Feature, configuration.Key, configuration.Value); err != nil {
		return nil, fmt.Errorf("invalid configuration %q for %q: %w", configuration.Key, configuration.Feature, err)
	}

	configuration.Secret = r.Features.IsSecret(configuration.Feature, configuration.Key)
	return r.Repo.ConfigCreate(ctx, configuration)
}

// ConfigurationUpdate is the resolver for the configurationUpdate field.
func (r *mutationResolver) ConfigurationUpdate(ctx context.Context, id uuid.UUID, configuration model.UpdateConfiguration) (model.Configuration, error) {
	return r.Repo.ConfigUpdate(ctx, id, configuration)
}

// ConfigurationDelete is the resolver for the configurationDelete field.
func (r *mutationResolver) ConfigurationDelete(ctx context.Context, id uuid.UUID) (bool, error) {
	// TODO(thokra): Make this soft delete?
	if err := r.Repo.ConfigDelete(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}

// Configuration is the resolver for the configuration field.
func (r *queryResolver) Configuration(ctx context.Context, feature string, envID *uuid.UUID) (*model.EnvConfig, error) {
	ret := &model.EnvConfig{}
	if envID != nil {
		// Get config for environment
		res, err := r.Repo.ConfigGetForEnv(ctx, feature, *envID)
		if err != nil {
			return nil, err
		}
		ret.Configuration = make([]model.Configuration, len(res))
		for i, c := range res {
			ret.Configuration[i] = c
		}
	}

	// Get global config
	res, err := r.Repo.ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}

OUTER2:
	for _, c := range res {
		for _, ec := range ret.Configuration {
			if ec.GetKey() == c.Key {
				continue OUTER2
			}
		}
		ret.Configuration = append(ret.Configuration, c)
	}

	f := r.Resolver.Features.Get(feature)
	if f == nil {
		return ret, nil
	}
OUTER:
	for key, val := range f.Config {
		for _, c := range ret.Configuration {
			if c.GetKey() == key {
				c.SetType(val.Type)
				c.SetDisplayName(val.DisplayName)
				c.SetDescription(val.Description)
				continue OUTER
			}
		}
		ret.Configuration = append(ret.Configuration, &model.GlobalConfiguration{
			FeatureName: feature,
			Key:         key,
			Value:       []byte("null"),
			Secret:      val.Secret,
			Type:        val.Type,
			DisplayName: val.DisplayName,
			Description: val.Description,
		})
	}

	if len(f.Mapping) == 0 || envID == nil {
		return ret, nil
	}

	mappingValues, err := r.Repo.MappingValuesForEnvironment(ctx, *envID, false)
	if err != nil {
		return nil, err
	}

	ret.Mapping, err = mappingToSlice(f, mappingValues)
	if err != nil {
		return nil, err
	}

	return ret, nil
}

// EnvConfig is the resolver for the envConfig field.
func (r *queryResolver) EnvConfig(ctx context.Context, feature string, envID uuid.UUID) (*model.EnvConfig, error) {
	config, err := r.Repo.EnvConfig(ctx, feature, envID)
	if err != nil {
		return nil, err
	}

	ret := &model.EnvConfig{
		Configuration: config,
	}

	f := r.Resolver.Features.Get(feature)

	if f == nil || len(f.Mapping) == 0 {
		return ret, nil
	}

	mappingValues, err := r.Repo.MappingValuesForEnvironment(ctx, envID, false)
	if err != nil {
		return nil, err
	}

	ret.Mapping, err = mappingToSlice(f, mappingValues)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// HelmValues is the resolver for the helmValues field.
func (r *queryResolver) HelmValues(ctx context.Context, feature string, envID uuid.UUID) (json.RawMessage, error) {
	f := r.Resolver.Features.Get(feature)
	if f == nil {
		return json.RawMessage{}, nil
	}

	v, err := r.Repo.HelmValues(ctx, *f, envID, nil)
	if err != nil {
		return nil, err
	}

	return json.Marshal(v)
}

// EnvConfiguration returns graphgen.EnvConfigurationResolver implementation.
func (r *Resolver) EnvConfiguration() graphgen.EnvConfigurationResolver {
	return &envConfigurationResolver{r}
}

// GlobalConfiguration returns graphgen.GlobalConfigurationResolver implementation.
func (r *Resolver) GlobalConfiguration() graphgen.GlobalConfigurationResolver {
	return &globalConfigurationResolver{r}
}

type envConfigurationResolver struct{ *Resolver }
type globalConfigurationResolver struct{ *Resolver }
