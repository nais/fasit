package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/graph/model"
)

// FeatureStatus is the resolver for the featureStatus field.
func (r *queryResolver) FeatureStatus(ctx context.Context, envID uuid.UUID, feature string) (*model.Status, error) {
	return r.Repo.StatusForFeature(ctx, envID, feature)
}

// Status is the resolver for the status field.
func (r *subscriptionResolver) Status(ctx context.Context, envID uuid.UUID, feature string) (<-chan *model.Status, error) {
	ch := make(chan *model.Status, 1)
	go func() {
		defer close(ch)
		for {
			s, err := r.Repo.StatusForFeature(ctx, envID, feature)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					r.Log.WithError(err).Debug("no rows returned from database")
					return
				}
				r.Log.WithError(err).Warn("failed to read status from status subscription")
				return
			}
			ch <- s
			select {
			case <-ctx.Done():
				r.Log.Debug("closing status subscription")
				return
			case <-time.After(5 * time.Second):
				// continue loop
			}
		}
	}()
	return ch, nil
}

// Subscription returns graphgen.SubscriptionResolver implementation.
func (r *Resolver) Subscription() graphgen.SubscriptionResolver { return &subscriptionResolver{r} }

type subscriptionResolver struct{ *Resolver }
