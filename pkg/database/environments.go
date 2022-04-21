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

func (r *repo) EnvironmentGet(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	env, err := r.querier.EnvironmentGet(ctx, id)
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func (r *repo) EnvironmentsGet(ctx context.Context, partnerID uuid.UUID) ([]*model.Environment, error) {
	envs, err := r.querier.EnvironmentsGet(ctx, partnerID)
	if err != nil {
		return nil, err
	}
	environmentSlice := []*model.Environment{}
	for _, env := range envs {
		environmentSlice = append(environmentSlice, environmentFromSQL(env))
	}
	return environmentSlice, nil
}

func (r *repo) EnvironmentCreate(ctx context.Context, p *model.EnvironmentCreate) (*model.Environment, error) {
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

func (r *repo) EnvironmentUpdate(ctx context.Context, environmentID uuid.UUID, p *model.EnvironmentUpdate) (*model.Environment, error) {
	partner, err := r.querier.EnvironmentUpdate(ctx, gensql.EnvironmentUpdateParams{
		Description: ptrToNullString(p.Description),
		ID:          environmentID,
	})
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(partner), nil
}

func (r *repo) EnvironmentIDByNames(ctx context.Context, partnerName, environmentName string) (uuid.UUID, error) {
	params := gensql.EnvironmentIDByNamesParams{
		EnvironmentName: environmentName,
		PartnerName:     partnerName,
	}
	return r.querier.EnvironmentIDByNames(ctx, params)
}
