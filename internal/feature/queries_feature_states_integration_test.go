//go:build integration_test

package feature

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
)

func TestFeatureStatesIntegration(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresql(ctx, t)

	tenantID := uuid.New()
	envID := uuid.New()
	fixtures := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'tenant1')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'dev', 'tenant')", envID, tenantID),
	}

	t.Run("Enable(new): creates state", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx := setupContext(pool)
		execQuery(ctx, t, pool, fixtures...)

		feat := &model.Feature{Name: "my-feature"}
		got, err := FeatureStatesEnable(ctx, envID, feat)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Enabled {
			t.Error("expected Enabled=true")
		}
		if got.FeatureName != "my-feature" {
			t.Errorf("FeatureName = %q, want %q", got.FeatureName, "my-feature")
		}
	})

	t.Run("Enable(already enabled): idempotent", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx := setupContext(pool)
		execQuery(ctx, t, pool, fixtures...)

		feat := &model.Feature{Name: "my-feature"}
		if _, err := FeatureStatesEnable(ctx, envID, feat); err != nil {
			t.Fatal(err)
		}
		got, err := FeatureStatesEnable(ctx, envID, feat)
		if err != nil {
			t.Fatal(err)
		}
		if !got.Enabled {
			t.Error("expected Enabled=true")
		}
	})

	t.Run("Disable(enabled): disables", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx := setupContext(pool)
		execQuery(ctx, t, pool, fixtures...)

		feat := &model.Feature{Name: "my-feature"}
		if _, err := FeatureStatesEnable(ctx, envID, feat); err != nil {
			t.Fatal(err)
		}
		got, err := FeatureStatesDisable(ctx, envID, feat, "broken")
		if err != nil {
			t.Fatal(err)
		}
		if got.Enabled {
			t.Error("expected Enabled=false")
		}
	})

	t.Run("Disable(blank reason): error", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx := setupContext(pool)
		execQuery(ctx, t, pool, fixtures...)

		feat := &model.Feature{Name: "my-feature"}
		if _, err := FeatureStatesEnable(ctx, envID, feat); err != nil {
			t.Fatal(err)
		}
		if _, err := FeatureStatesDisable(ctx, envID, feat, "  "); err == nil {
			t.Error("expected error for blank reason")
		}
	})

	t.Run("Disable(already disabled): no-op", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx := setupContext(pool)
		execQuery(ctx, t, pool, fixtures...)

		feat := &model.Feature{Name: "my-feature"}
		// Enable then disable to get a disabled row in DB.
		if _, err := FeatureStatesEnable(ctx, envID, feat); err != nil {
			t.Fatal(err)
		}
		if _, err := FeatureStatesDisable(ctx, envID, feat, "initial"); err != nil {
			t.Fatal(err)
		}
		// Second disable is a no-op.
		got, err := FeatureStatesDisable(ctx, envID, feat, "")
		if err != nil {
			t.Fatal(err)
		}
		if got.Enabled {
			t.Error("expected Enabled=false")
		}
	})

	t.Run("Enable(missing dependency): error", func(t *testing.T) {
		pool := newPool(ctx, t, container, dsn)
		ctx := setupContext(pool)
		execQuery(ctx, t, pool, fixtures...)

		feat := &model.Feature{
			Name: "dependent",
			FeatureYAML: model.FeatureYAML{
				Dependencies: model.Dependencies{
					&model.Dependency{AllOf: []string{"base-feature"}},
				},
			},
		}
		_, err := FeatureStatesEnable(ctx, envID, feat)
		if err == nil {
			t.Error("expected error for missing dependency")
		}
	})
}
