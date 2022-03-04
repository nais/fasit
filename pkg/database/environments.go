package database

import (
	"context"

	"github.com/nais/c3po/pkg/database/gensql"
	"github.com/nais/c3po/pkg/graph/model"
)

func environmentFromSQL(p gensql.Environment) *model.Environment {
	return &model.Environment{
		ID:           p.ID,
		Name:         p.Name,
		Description:  nullStringToPtr(p.Description),
		Created:      p.Created,
		LastModified: p.LastModified,
	}
}

func (r *Repo) EnvironmentCreate(ctx context.Context, p *model.EnvironmentCreate) (*model.Environment, error) {
	partner, err := r.querier.EnvironmentCreate(ctx, gensql.EnvironmentCreateParams{
		Name:        p.Name,
		Description: ptrToNullString(p.Description),
		PartnerID:   p.PartnerID,
	})
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(partner), nil
}