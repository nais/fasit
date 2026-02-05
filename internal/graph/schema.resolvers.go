package graph

import (
	"github.com/nais/fasit/internal/graph/graphgen"
)

func (r *Resolver) Mutation() graphgen.MutationResolver { return &mutationResolver{r} }

func (r *Resolver) Query() graphgen.QueryResolver { return &queryResolver{r} }

func (r *Resolver) Subscription() graphgen.SubscriptionResolver { return &subscriptionResolver{r} }

type (
	mutationResolver     struct{ *Resolver }
	queryResolver        struct{ *Resolver }
	subscriptionResolver struct{ *Resolver }
)
