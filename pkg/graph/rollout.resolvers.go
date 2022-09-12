package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// Feature is the resolver for the feature field.
func (r *rolloutResolver) Feature(ctx context.Context, obj *model.Rollout) (*model.Feature, error) {
	panic(fmt.Errorf("not implemented: Feature - feature"))
}

// New is the resolver for the New field.
func (r *rolloutChangesetResolver) New(ctx context.Context, obj *model.RolloutChangeset) (json.RawMessage, error) {
	panic(fmt.Errorf("not implemented: New - New"))
}

// Old is the resolver for the Old field.
func (r *rolloutChangesetResolver) Old(ctx context.Context, obj *model.RolloutChangeset) (json.RawMessage, error) {
	panic(fmt.Errorf("not implemented: Old - Old"))
}

// Rollout returns graphgen.RolloutResolver implementation.
func (r *Resolver) Rollout() graphgen.RolloutResolver { return &rolloutResolver{r} }

// RolloutChangeset returns graphgen.RolloutChangesetResolver implementation.
func (r *Resolver) RolloutChangeset() graphgen.RolloutChangesetResolver {
	return &rolloutChangesetResolver{r}
}

type rolloutResolver struct{ *Resolver }
type rolloutChangesetResolver struct{ *Resolver }
