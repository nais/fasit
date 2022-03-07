package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
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

func (r *Repo) EnvironmentIDByNames(ctx context.Context, partnerName, environmentName string) (uuid.UUID, error) {
	params := gensql.EnvironmentIDByNamesParams{
		EnvironmentName: environmentName,
		PartnerName:     partnerName,
	}
	return r.querier.EnvironmentIDByNames(ctx, params)
}
