//go:build integration_test

package featureassignment_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
)

func TestDeactivation(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}

	setupCtx := func(t *testing.T) (context.Context, *featureassignmenttest.Seeder) {
		t.Helper()
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("create loader: %v", err)
		}
		return loadContext(ctx), mgr.seeder
	}

	t.Run("creating deployment deactivates previous for same target", func(t *testing.T) {
		ctx, seeder := setupCtx(t)

		seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant"})
		ids, err := seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}

		dep1, err := featureassignment.Get(ctx, ids[0])
		if err != nil {
			t.Fatalf("get dep1: %v", err)
		}
		if dep1.Active {
			t.Error("old deployment should be inactive")
		}

		dep2, err := featureassignment.Get(ctx, ids[1])
		if err != nil {
			t.Fatalf("get dep2: %v", err)
		}
		if !dep2.Active {
			t.Error("new deployment should be active")
		}
	})

	t.Run("creating deployment does not deactivate different target", func(t *testing.T) {
		ctx, seeder := setupCtx(t)

		seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant", "name": "dev"})
		ids, err := seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}

		dep1, err := featureassignment.Get(ctx, ids[0])
		if err != nil {
			t.Fatalf("get dep1: %v", err)
		}
		if !dep1.Active {
			t.Error("broad deployment should remain active")
		}

		dep2, err := featureassignment.Get(ctx, ids[1])
		if err != nil {
			t.Fatalf("get dep2: %v", err)
		}
		if !dep2.Active {
			t.Error("specific deployment should be active")
		}
	})

	t.Run("deactivate sets deployment inactive", func(t *testing.T) {
		ctx, seeder := setupCtx(t)

		seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		ids, err := seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}

		if _, err := featureassignment.Deactivate(ctx, ids[0]); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		dep, err := featureassignment.Get(ctx, ids[0])
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		if dep.Active {
			t.Error("deactivated deployment should be inactive")
		}
	})

	t.Run("ListByFeature returns only active deployments", func(t *testing.T) {
		ctx, seeder := setupCtx(t)

		seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant"})
		seeder.AddAssignment("myapp", "3.0.0", environment.Labels{"kind": "tenant", "name": "dev"})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seed: %v", err)
		}

		deps, err := featureassignment.ListByFeature(ctx, "myapp")
		if err != nil {
			t.Fatalf("list: %v", err)
		}

		if len(deps) != 2 {
			t.Fatalf("expected 2 active deployments, got %d", len(deps))
		}
		for _, dep := range deps {
			if !dep.Active {
				t.Errorf("ListByFeature returned inactive deployment %s", dep.ID)
			}
		}
	})

	t.Run("ListAllByFeature returns active and inactive", func(t *testing.T) {
		ctx, seeder := setupCtx(t)

		seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant"})
		if _, err := seeder.Seed(ctx); err != nil {
			t.Fatalf("seed: %v", err)
		}

		deps, err := featureassignment.ListAllByFeature(ctx, "myapp")
		if err != nil {
			t.Fatalf("list: %v", err)
		}

		if len(deps) != 2 {
			t.Fatalf("expected 2 deployments, got %d", len(deps))
		}

		var active, inactive int
		for _, dep := range deps {
			if dep.Active {
				active++
			} else {
				inactive++
			}
		}
		if active != 1 || inactive != 1 {
			t.Errorf("ListAllByFeature() = %d active + %d inactive, want 1 + 1", active, inactive)
		}
	})

	t.Run("deactivated deployment not visible in FeatureForEnvironment", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("create loader: %v", err)
		}
		tCtx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(tCtx, map[string]environment.Labels{
			"nav:dev": {"kind": "tenant"},
		})

		mgr.seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		ids, err := mgr.seeder.Seed(tCtx)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}

		if _, err := featureassignment.Deactivate(tCtx, ids[0]); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		tenant, err := environment.GetTenantByName(tCtx, "nav")
		if err != nil {
			t.Fatalf("get tenant: %v", err)
		}

		env, err := environment.GetByName(tCtx, tenant.ID, "dev")
		if err != nil {
			t.Fatalf("get env: %v", err)
		}

		_, err = featureassignment.FeatureForEnvironment(tCtx, env.ID, "myapp")
		if !errors.Is(err, featureassignment.ErrFeatureNotFound) {
			t.Errorf("FeatureForEnvironment() err = %v, want ErrFeatureNotFound", err)
		}
	})

	t.Run("ListAll returns one row per feature+target preferring active", func(t *testing.T) {
		ctx, seeder := setupCtx(t)

		// Two versions for same target — v1 gets deactivated, v2 stays active
		seeder.AddAssignment("myapp", "1.0.0", environment.Labels{"kind": "tenant"})
		seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"kind": "tenant"})
		// Different target — stays active
		seeder.AddAssignment("myapp", "3.0.0", environment.Labels{"kind": "tenant", "name": "dev"})
		// Different feature, deactivated (no active replacement)
		seeder.AddAssignment("otherapp", "1.0.0", environment.Labels{"kind": "tenant"})
		ids, err := seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("seed: %v", err)
		}

		// Deactivate otherapp so it has no active deployment
		if _, err := featureassignment.Deactivate(ctx, ids[3]); err != nil {
			t.Fatalf("deactivate: %v", err)
		}

		deps, err := featureassignment.ListAll(ctx)
		if err != nil {
			t.Fatalf("ListAll: %v", err)
		}

		// Expect 3 rows: myapp+broad(active v2), myapp+dev(active v3), otherapp+broad(inactive v1)
		if len(deps) != 3 {
			t.Fatalf("ListAll() returned %d rows, want 3", len(deps))
		}

		type key struct{ feature, version string }
		got := make(map[key]bool)
		for _, dep := range deps {
			got[key{dep.Feature.Name, dep.Feature.Version}] = dep.Active
		}

		// myapp v2 (active, broad target)
		if active, ok := got[key{"myapp", "2.0.0"}]; !ok || !active {
			t.Errorf("expected myapp v2.0.0 active, got ok=%v active=%v", ok, active)
		}
		// myapp v3 (active, dev target)
		if active, ok := got[key{"myapp", "3.0.0"}]; !ok || !active {
			t.Errorf("expected myapp v3.0.0 active, got ok=%v active=%v", ok, active)
		}
		// otherapp v1 (inactive, only version)
		if active, ok := got[key{"otherapp", "1.0.0"}]; !ok || active {
			t.Errorf("expected otherapp v1.0.0 inactive, got ok=%v active=%v", ok, active)
		}
		// myapp v1 should NOT appear (superseded by v2 for same target)
		if _, ok := got[key{"myapp", "1.0.0"}]; ok {
			t.Error("ListAll() should not return myapp v1.0.0 (superseded)")
		}
	})
}
