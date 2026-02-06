package graph

import (
	"context"

	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
)

// Feature is the resolver for the feature field.
func (r *featureWarningResolver) Feature(ctx context.Context, obj *model.FeatureWarning) (*model.Feature, error) {
	return featurepkg.FeatureByNameForEnv(ctx, obj.FeatureName, obj.EnvironmentID)
}

// Environment is the resolver for the environment field.
func (r *featureWarningResolver) Environment(ctx context.Context, obj *model.FeatureWarning) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

// Environment is the resolver for the environment field.
func (r *naisdWarningResolver) Environment(ctx context.Context, obj *model.NaisdWarning) (*model.Environment, error) {
	return r.Repo.EnvironmentGet(ctx, obj.EnvironmentID)
}

func (r *Resolver) FeatureWarning() graphgen.FeatureWarningResolver {
	return &featureWarningResolver{r}
}

func (r *Resolver) NaisdWarning() graphgen.NaisdWarningResolver { return &naisdWarningResolver{r} }

type (
	featureWarningResolver struct{ *Resolver }
	naisdWarningResolver   struct{ *Resolver }
)
