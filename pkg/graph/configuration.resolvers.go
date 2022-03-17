package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *mutationResolver) ConfigurationCreate(ctx context.Context, configuration model.NewConfiguration) (*model.Configuration, error) {
	if err := r.Features.ValidConfig(configuration.Feature, configuration.Key, configuration.Value); err != nil {
		return nil, fmt.Errorf("invalid configuration %q for %q: %w", configuration.Key, configuration.Feature, err)
	}

	configuration.Secret = r.Features.IsSecret(configuration.Feature, configuration.Key)
	return r.Repo.ConfigCreate(ctx, configuration)
}

func (r *queryResolver) Configuration(ctx context.Context, feature string, envID *uuid.UUID) ([]*model.Configuration, error) {
	if envID != nil {
		return r.Repo.ConfigGetForEnv(ctx, feature, *envID)
	}
	ret, err := r.Repo.ConfigGet(ctx, feature)
	if err != nil {
		return nil, err
	}

	f := r.Resolver.Features.Get(feature)
	if f == nil {
		return ret, nil
	}

OUTER:
	for key, val := range f.Config {
		for _, c := range ret {
			if c.Key == key {
				c.Type = val.Type
				continue OUTER
			}
		}
		ret = append(ret, &model.Configuration{
			Feature: feature,
			Key:     key,
			Value:   []byte("null"),
			Secret:  val.Secret,
			Type:    val.Type,
		})
	}

	return ret, nil
}
