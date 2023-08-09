package integration_test

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	integration "github.com/nais/fasit/pkg/integration_test"
)

type seedTenantEnvs struct {
	*model.Tenant
	Env map[string]*model.Environment
}

func seedTenantEnv(ctx context.Context, db database.Repo, state map[string]any, config *integration.Config) error {
	if config == nil {
		return fmt.Errorf("no config defined. Does the test have a `00_config.yaml`?")
	}
	tenantState := map[string]*seedTenantEnvs{}
	for _, t := range config.Tenants {
		tenant, err := seedTenant(ctx, db, t)
		if err != nil {
			return err
		}
		tenantState[t.Name] = tenant
	}

	state["Tenant"] = tenantState
	return nil
}

func seedTenant(ctx context.Context, db database.Repo, tenant integration.Tenant) (*seedTenantEnvs, error) {
	t, err := db.TenantCreate(ctx, &model.TenantCreate{
		Name: tenant.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("seedTenantEnv: unable to create tenant %v: %w", tenant.Name, err)
	}

	if tenant.CI {
		_, tx, err := db.WithTx(ctx)
		if err != nil {
			return nil, fmt.Errorf("seedTenantEnv: unable to create tenant %v transaction: %w", tenant.Name, err)
		}
		err = pgx.BeginFunc(ctx, tx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE tenants SET ci = true WHERE id = $1`, t.ID); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			tx.Rollback(ctx)
			return nil, err
		}
		if err := tx.Commit(ctx); err != nil {
			return nil, err
		}
	}

	ret := &seedTenantEnvs{
		Tenant: t,
		Env:    map[string]*model.Environment{},
	}

	for _, e := range tenant.Envs {
		env, err := seedEnvironment(ctx, db, t, e)
		if err != nil {
			return nil, err
		}
		ret.Env[env.Name] = env
	}
	return ret, nil
}

func seedEnvironment(ctx context.Context, db database.Repo, tenant *model.Tenant, env integration.Env) (*model.Environment, error) {
	e, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     env.Name,
		Kind:     env.Kind,
		TenantID: tenant.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("seedTenantEnv: unable to create %v environment: %w", env.Name, err)
	}

	e, err = db.EnvironmentSetReconcile(ctx, e.ID, env.Reconcile)
	if err != nil {
		return nil, fmt.Errorf("seedTenantEnv: unable to set reconcile for %v environment: %w", env.Name, err)
	}

	if env.CI {
		_, tx, err := db.WithTx(ctx)
		if err != nil {
			return nil, fmt.Errorf("seedTenantEnv: unable to create environment %v transaction: %w", env.Name, err)
		}
		err = pgx.BeginFunc(ctx, tx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE environments SET ci = true WHERE id = $1`, e.ID); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			tx.Rollback(ctx)
			return nil, err
		}
		return e, tx.Commit(ctx)
	}

	return e, nil
}
