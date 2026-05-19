//go:build integration_test

package deployment_test

import (
	"context"
	"sort"
	"testing"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValueRefsForEnvironment(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t)
	require.NoError(t, err)

	t.Run("returns feature names referencing each env key", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx = loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		// Feature "app-a" references env keys "DB_HOST" and "API_KEY"
		mgr.seeder.AddDeploymentWithValues(
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
		mgr.seeder.AddDeploymentWithValues(
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
		mgr.seeder.AddDeployment("app-c", "1.0.0", environment.Labels{})

		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		refs, err := deployment.ValueRefsForEnvironment(ctx, env.ID)
		require.NoError(t, err)

		sortRefs(refs)

		assert.Equal(t, []string{"app-a", "app-b"}, refs["DB_HOST"])
		assert.Equal(t, []string{"app-a"}, refs["API_KEY"])
		assert.Equal(t, []string{"app-b"}, refs["LOG_LEVEL"])
		assert.Nil(t, refs["NONEXISTENT"])
	})

	t.Run("includes disabled features", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx = loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		mgr.seeder.AddDeploymentWithValues(
			"app-a", "1.0.0",
			environment.Labels{},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"val": {Computed: &model.Computed{Template: `{{ .Env.SECRET }}`}},
			},
			nil, "",
		)

		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		// Disable the feature
		err = featurepkg.FeatureDisable(ctx, env.ID, "app-a", "test")
		require.NoError(t, err)

		refs, err := deployment.ValueRefsForEnvironment(ctx, env.ID)
		require.NoError(t, err)

		assert.Equal(t, []string{"app-a"}, refs["SECRET"], "disabled features should still appear in refs")
	})

	t.Run("deduplicates when multiple deployments target same env", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx = loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"extra": "yes"},
		})

		// Two deployments for same feature with different targets that both match nav:dev
		mgr.seeder.AddDeploymentWithValues(
			"app-a", "1.0.0",
			environment.Labels{},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"val": {Computed: &model.Computed{Template: `{{ .Env.SHARED_KEY }}`}},
			},
			nil, "",
		)
		mgr.seeder.AddDeploymentWithValues(
			"app-a", "2.0.0",
			environment.Labels{"extra": "yes"},
			[]model.EnvironmentKind{"tenant"},
			model.Values{
				"val": {Computed: &model.Computed{Template: `{{ .Env.SHARED_KEY }}`}},
			},
			nil, "",
		)

		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		refs, err := deployment.ValueRefsForEnvironment(ctx, env.ID)
		require.NoError(t, err)

		assert.Equal(t, []string{"app-a"}, refs["SHARED_KEY"], "same feature should appear only once")
	})
}

func sortRefs(refs map[string][]string) {
	for _, names := range refs {
		sort.Strings(names)
	}
}
