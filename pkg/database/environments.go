package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type EnvironmentRepo interface {
	EnvironmentByNames(ctx context.Context, tenantName, environmentName string) (*model.Environment, error)
	EnvironmentCI(ctx context.Context, kind model.EnvironmentKind) (*model.Environment, error)
	EnvironmentCreate(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error)
	EnvironmentGet(ctx context.Context, id uuid.UUID) (*model.Environment, error)
	EnvironmentGetByName(ctx context.Context, tenantID uuid.UUID, name string) (*model.Environment, error)
	EnvironmentIDByNames(ctx context.Context, tenantName, environmentName string) (uuid.UUID, error)
	EnvironmentsGet(ctx context.Context, tenantID uuid.UUID) ([]*model.Environment, error)
	EnvironmentUpdate(ctx context.Context, environmentID uuid.UUID, p *model.EnvironmentUpdate) (*model.Environment, error)
	EnvironmentSetReconcile(ctx context.Context, environmentID uuid.UUID, reconcile bool) (*model.Environment, error)
}

func environmentFromSQL(p gensql.Environment) *model.Environment {
	return &model.Environment{
		ID:           p.ID,
		Name:         p.Name,
		Description:  nullStringToPtr(p.Description),
		Created:      p.Created,
		LastModified: p.LastModified,
		Kind:         model.EnvironmentKind(p.Kind),
		TenantID:     p.TenantID,
		CI:           p.Ci,
		Reconcile:    p.Reconcile,
	}
}

func (r *repo) EnvironmentGet(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	env, err := r.querier.EnvironmentGet(ctx, id)
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func (r *repo) EnvironmentGetByName(ctx context.Context, tenantID uuid.UUID, name string) (*model.Environment, error) {
	env, err := r.querier.EnvironmentGetByName(ctx, gensql.EnvironmentGetByNameParams{
		TenantID: tenantID,
		Name:     name,
	})
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(env), nil
}

func (r *repo) EnvironmentsGet(ctx context.Context, tenantID uuid.UUID) ([]*model.Environment, error) {
	envs, err := r.querier.EnvironmentsGet(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	environmentSlice := []*model.Environment{}
	for _, env := range envs {
		environmentSlice = append(environmentSlice, environmentFromSQL(env))
	}
	return environmentSlice, nil
}

func (r *repo) EnvironmentCreate(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error) {
	env, err := r.querier.EnvironmentCreate(ctx, gensql.EnvironmentCreateParams{
		Name:        t.Name,
		Description: ptrToNullString(t.Description),
		TenantID:    t.TenantID,
		Kind:        gensql.EnvironmentKind(t.Kind),
	})
	if err != nil {
		return nil, err
	}

	r.createAudit(ctx, "created", "environments", env.ID.String())

	return environmentFromSQL(env), nil
}

func (r *repo) EnvironmentUpdate(ctx context.Context, environmentID uuid.UUID, p *model.EnvironmentUpdate) (*model.Environment, error) {
	env, err := r.querier.EnvironmentUpdate(ctx, gensql.EnvironmentUpdateParams{
		Description: ptrToNullString(p.Description),
		ID:          environmentID,
	})
	if err != nil {
		return nil, err
	}

	r.createAudit(ctx, "environment updated", "environments", env.ID.String())

	return environmentFromSQL(env), nil
}

func (r *repo) EnvironmentByNames(ctx context.Context, tenantName, environmentName string) (*model.Environment, error) {
	params := gensql.EnvironmentByNamesParams{
		EnvironmentName: environmentName,
		TenantName:      tenantName,
	}
	res, err := r.querier.EnvironmentByNames(ctx, params)
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(res), nil
}

func (r *repo) EnvironmentIDByNames(ctx context.Context, tenantName, environmentName string) (uuid.UUID, error) {
	params := gensql.EnvironmentIDByNamesParams{
		EnvironmentName: environmentName,
		TenantName:      tenantName,
	}
	return r.querier.EnvironmentIDByNames(ctx, params)
}

func (r *repo) EnvironmentCI(ctx context.Context, kind model.EnvironmentKind) (*model.Environment, error) {
	res, err := r.querier.EnvironmentCI(ctx, gensql.EnvironmentKind(kind))
	if err != nil {
		return nil, err
	}
	return environmentFromSQL(res), nil
}

func (r *repo) EnvironmentSetReconcile(ctx context.Context, environmentID uuid.UUID, reconcile bool) (*model.Environment, error) {
	env, err := r.querier.EnvironmentSetReconcile(ctx, gensql.EnvironmentSetReconcileParams{
		ID:        environmentID,
		Reconcile: reconcile,
	})
	if err != nil {
		return nil, err
	}

	txt := "enabled"
	if !reconcile {
		txt = "disabled"
	}

	r.createAudit(ctx, "environment reconcile "+txt, "environments", env.ID.String())

	return environmentFromSQL(env), nil
}
