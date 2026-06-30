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
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
)

func TestFeatureForEnvironment(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("returns most-specific assignment feature", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"team": "myteam"},
		})

		// Broad assignment (no target labels) with version 1.0.0
		mgr.seeder.AddAssignment("myapp", "1.0.0", environment.Labels{})
		// More specific assignment targeting team=myteam with version 2.0.0
		mgr.seeder.AddAssignment("myapp", "2.0.0", environment.Labels{"team": "myteam"})

		_, err = mgr.seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tenant, err := environment.GetTenantByName(ctx, "nav")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		feat, err := featureassignment.FeatureForEnvironment(ctx, env.ID, "myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if feat.Name != "myapp" {
			t.Errorf("got name %q, want myapp", feat.Name)
		}
		if feat.Version != "2.0.0" {
			t.Errorf("should pick the more specific assignment: got %q", feat.Version)
		}
	})

	t.Run("returns ErrFeatureNotFound when no assignment exists", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		tenant, err := environment.GetTenantByName(ctx, "nav")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = featureassignment.FeatureForEnvironment(ctx, env.ID, "nonexistent")
		if !errors.Is(err, featureassignment.ErrFeatureNotFound) {
			t.Errorf("got err %v, want ErrFeatureNotFound", err)
		}
	})

	t.Run("returns feature even when disabled", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		mgr.seeder.AddAssignment("myapp", "1.0.0", environment.Labels{})
		_, err = mgr.seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tenant, err := environment.GetTenantByName(ctx, "nav")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = featurepkg.FeatureDisable(ctx, env.ID, "myapp", "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		feat, err := featureassignment.FeatureForEnvironment(ctx, env.ID, "myapp")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if feat.Name != "myapp" {
			t.Errorf("disabled features should still be returned: got %q", feat.Name)
		}
	})
}

func TestListEnvironmentFeatures(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("reports FeatureDisabled correctly", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		mgr.seeder.AddAssignment("enabled-app", "1.0.0", environment.Labels{})
		mgr.seeder.AddAssignment("disabled-app", "1.0.0", environment.Labels{})

		_, err = mgr.seeder.Seed(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		tenant, err := environment.GetTenantByName(ctx, "nav")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = featurepkg.FeatureDisable(ctx, env.ID, "disabled-app", "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assignments, err := featureassignment.ListForEnvironment(ctx, env.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		featureMap := make(map[string]bool, len(assignments))
		for _, a := range assignments {
			featureMap[a.Feature.Name] = a.FeatureDisabled
		}

		if featureMap["enabled-app"] {
			t.Errorf("enabled-app should not be disabled")
		}
		if !featureMap["disabled-app"] {
			t.Errorf("disabled-app should be disabled")
		}
	})

	t.Run("returns empty list when no assignments match", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		tenant, err := environment.GetTenantByName(ctx, "nav")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		assignments, err := featureassignment.ListForEnvironment(ctx, env.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(assignments) != 0 {
			t.Errorf("got %d assignments, want 0", len(assignments))
		}
	})
}
