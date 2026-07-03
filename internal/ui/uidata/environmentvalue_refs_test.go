//go:build integration_test

package uidata_test

/*
import (
	"context"
	"io"
	"log/slog"
	"sort"
	"testing"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
)


// TODO: rewrite to test refs only, no integration test necessary, assignments are tested elsewhere
func TestValueRefsForEnvironment(t *testing.T) {
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("returns feature names referencing each env key", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx = loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		// Feature "app-a" references env keys "DB_HOST" and "API_KEY"
		mgr.seeder.AddAssignmentWithValues(
			"app-a", "1.0.0",
			environment.Labels{},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"db_url": {Computed: &model.Computed{Template: `{{ .Env.DB_HOST }}`}},
				"key":    {Computed: &model.Computed{Template: `{{ .Env.API_KEY }}`}},
			},
			nil, "",
		)

		// Feature "app-b" references "DB_HOST" (overlap with app-a) and "LOG_LEVEL"
		mgr.seeder.AddAssignmentWithValues(
			"app-b", "1.0.0",
			environment.Labels{},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"db":  {Computed: &model.Computed{Template: `{{ .Env.DB_HOST }}`}},
				"log": {Computed: &model.Computed{Template: `{{ .Env.LOG_LEVEL }}`}},
			},
			nil, "",
		)

		// Feature "app-c" has no computed values (no env refs)
		mgr.seeder.AddAssignment("app-c", "1.0.0", environment.Labels{})

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

		refs, err := featureassignment.GetEnvironmentValueReferences(ctx, env.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		sortRefs(refs)

		if got := refs["DB_HOST"]; len(got) != 2 || got[0] != "app-a" || got[1] != "app-b" {
			t.Errorf("DB_HOST = %v, want [app-a app-b]", got)
		}
		if got := refs["API_KEY"]; len(got) != 1 || got[0] != "app-a" {
			t.Errorf("API_KEY = %v, want [app-a]", got)
		}
		if got := refs["LOG_LEVEL"]; len(got) != 1 || got[0] != "app-b" {
			t.Errorf("LOG_LEVEL = %v, want [app-b]", got)
		}
		if refs["NONEXISTENT"] != nil {
			t.Errorf("NONEXISTENT = %v, want nil", refs["NONEXISTENT"])
		}
	})

	t.Run("includes disabled features", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx = loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		mgr.seeder.AddAssignmentWithValues(
			"app-a", "1.0.0",
			environment.Labels{},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"val": {Computed: &model.Computed{Template: `{{ .Env.SECRET }}`}},
			},
			nil, "",
		)

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

		// Disable the feature
		err = featurepkg.DisableFeature(ctx, env.ID, "app-a", "test")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		refs, err := featureassignment.GetEnvironmentValueReferences(ctx, env.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := refs["SECRET"]; len(got) != 1 || got[0] != "app-a" {
			t.Errorf("SECRET = %v, want [app-a] (disabled features should still appear)", got)
		}
	})

	t.Run("deduplicates when multiple deployments target same env", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		featureassignment.ChartDownloader = mgr.seeder.ChartDownloader()

		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, logger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ctx = loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"extra": "yes"},
		})

		// Two deployments for same feature with different targets that both match nav:dev
		mgr.seeder.AddAssignmentWithValues(
			"app-a", "1.0.0",
			environment.Labels{},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"val": {Computed: &model.Computed{Template: `{{ .Env.SHARED_KEY }}`}},
			},
			nil, "",
		)
		mgr.seeder.AddAssignmentWithValues(
			"app-a", "2.0.0",
			environment.Labels{"extra": "yes"},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"val": {Computed: &model.Computed{Template: `{{ .Env.SHARED_KEY }}`}},
			},
			nil, "",
		)

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

		refs, err := featureassignment.GetEnvironmentValueReferences(ctx, env.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got := refs["SHARED_KEY"]; len(got) != 1 || got[0] != "app-a" {
			t.Errorf("SHARED_KEY = %v, want [app-a] (same feature should appear only once)", got)
		}
	})
}

func sortRefs(refs map[string][]string) {
	for _, names := range refs {
		sort.Strings(names)
	}
}
*/
