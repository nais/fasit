package graph

import (
	"context"

	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/graph/model"
)

// UserInfo is the resolver for the userInfo field.
func (r *queryResolver) UserInfo(ctx context.Context) (*model.UserInfo, error) {
	return &model.UserInfo{
		Email: auth.GetEmail(ctx),
	}, nil
}
