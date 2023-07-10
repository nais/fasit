package integration_test

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/integration_test/testmanager"
)

func seedTenantEnv(ctx context.Context, db database.Repo, state map[string]any, config testmanager.Config) error {
	if v, ok := config.Bool("no_tenants"); ok && v {
		return nil
	}

	tenant, err := db.TenantCreate(ctx, &model.TenantCreate{
		Name: tenantName,
	})
	if err != nil {
		return fmt.Errorf("seedTenantEnv: unable to create tenant: %w", err)
	}

	mgmt, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     envManagementName,
		Kind:     model.EnvironmentKindManagement,
		TenantID: tenant.ID,
	})
	if err != nil {
		return fmt.Errorf("seedTenantEnv: unable to create management environment: %w", err)
	}

	tenantEnv, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     envTenantName,
		Kind:     model.EnvironmentKindTenant,
		TenantID: tenant.ID,
	})
	if err != nil {
		return fmt.Errorf("seedTenantEnv: unable to create environment: %w", err)
	}

	nonCIEnv, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     envTenantNonCI,
		Kind:     model.EnvironmentKindTenant,
		TenantID: tenant.ID,
	})
	if err != nil {
		return fmt.Errorf("seedTenantEnv: unable to create environment: %w", err)
	}

	state["tenant"] = tenant
	state["env"] = tenantEnv
	state["nonCIEnv"] = nonCIEnv
	state["mgmt"] = mgmt

	if v, _ := config.Bool("ci"); v {
		_, tx, err := db.WithTx(ctx)
		if err != nil {
			return fmt.Errorf("seedTenantEnv: unable to create transaction: %w", err)
		}
		err = pgx.BeginFunc(ctx, tx, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE tenants SET ci = true`); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE environments SET ci = true WHERE name != $1`, nonCIEnv.Name); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			tx.Rollback(ctx)
			return err
		}
		return tx.Commit(ctx)
	}

	return nil
}
