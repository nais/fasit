package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"fmt"

	"github.com/nais/fasit/pkg/graph/model"
)

func (r *mutationResolver) ConfigurationCreate(ctx context.Context, configuration model.NewConfiguration) (*model.Configuration, error) {
	if !r.Features.ValidConfig(configuration.Feature, configuration.Key, configuration.Value) {
		return nil, fmt.Errorf("invalid configuration %q for %q", configuration.Key, configuration.Feature)
	}

	configuration.Secret = r.Features.IsSecret(configuration.Feature, configuration.Key)
	return r.Repo.ConfigCreate(ctx, configuration)
}

func (r *queryResolver) Configuration(ctx context.Context, feature string) (*model.Configuration, error) {
	return r.Repo.ConfigGet(ctx, feature)
}
