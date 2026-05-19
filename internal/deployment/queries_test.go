//go:build integration_test

package deployment_test

import (
	"context"
	"testing"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureForEnvironment(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t)
	require.NoError(t, err)

	t.Run("returns most-specific deployment feature", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"team": "myteam"},
		})

		// Broad deployment (no target labels) with version 1.0.0
		mgr.seeder.AddDeployment("myapp", "1.0.0", environment.Labels{})
		// More specific deployment targeting team=myteam with version 2.0.0
		mgr.seeder.AddDeployment("myapp", "2.0.0", environment.Labels{"team": "myteam"})

		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		feat, err := deployment.FeatureForEnvironment(ctx, env.ID, "myapp")
		require.NoError(t, err)

		assert.Equal(t, "myapp", feat.Name)
		assert.Equal(t, "2.0.0", feat.Version, "should pick the more specific deployment")
	})

	t.Run("returns ErrFeatureNotFound when no deployment exists", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		_, err = deployment.FeatureForEnvironment(ctx, env.ID, "nonexistent")
		assert.ErrorIs(t, err, deployment.ErrFeatureNotFound)
	})

	t.Run("returns feature even when disabled", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		mgr.seeder.AddDeployment("myapp", "1.0.0", environment.Labels{})
		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		err = featurepkg.FeatureDisable(ctx, env.ID, "myapp", "test")
		require.NoError(t, err)

		feat, err := deployment.FeatureForEnvironment(ctx, env.ID, "myapp")
		require.NoError(t, err)
		assert.Equal(t, "myapp", feat.Name, "disabled features should still be returned")
	})
}

func TestListEnvironmentFeatures(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t)
	require.NoError(t, err)

	t.Run("returns sorted deduplicated feature names", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {"team": "myteam"},
		})

		mgr.seeder.AddDeployment("charlie", "1.0.0", environment.Labels{})
		mgr.seeder.AddDeployment("alpha", "1.0.0", environment.Labels{})
		mgr.seeder.AddDeployment("bravo", "1.0.0", environment.Labels{})
		// Second deployment for alpha with more specific target — should not duplicate
		mgr.seeder.AddDeployment("alpha", "2.0.0", environment.Labels{"team": "myteam"})

		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		features, err := deployment.ListEnvironmentFeatures(ctx, env.ID)
		require.NoError(t, err)

		names := make([]string, len(features))
		for i, f := range features {
			names[i] = f.Name
		}
		assert.Equal(t, []string{"alpha", "bravo", "charlie"}, names)

		for _, f := range features {
			assert.False(t, f.FeatureDisabled, "no features should be disabled")
		}
	})

	t.Run("reports FeatureDisabled correctly", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		mgr.seeder.AddDeployment("enabled-app", "1.0.0", environment.Labels{})
		mgr.seeder.AddDeployment("disabled-app", "1.0.0", environment.Labels{})

		_, err = mgr.seeder.Seed(ctx)
		require.NoError(t, err)

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		err = featurepkg.FeatureDisable(ctx, env.ID, "disabled-app", "test")
		require.NoError(t, err)

		features, err := deployment.ListEnvironmentFeatures(ctx, env.ID)
		require.NoError(t, err)

		featureMap := make(map[string]bool, len(features))
		for _, f := range features {
			featureMap[f.Name] = f.FeatureDisabled
		}

		assert.False(t, featureMap["enabled-app"], "enabled-app should not be disabled")
		assert.True(t, featureMap["disabled-app"], "disabled-app should be disabled")
	})

	t.Run("returns empty list when no deployments match", func(t *testing.T) {
		mgr := setupTestMgr(ctx, t, container, dsn, logger)
		deployment.ChartDownloader = mgr.seeder.ChartDownloader()

		newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
			return mgr.publisher
		}
		loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, meter, logger)
		require.NoError(t, err)
		ctx := loadContext(ctx)

		mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
			"nav:dev": {},
		})

		tenant, err := environment.GetTenantByName(ctx, "nav")
		require.NoError(t, err)

		env, err := environment.GetByName(ctx, tenant.ID, "dev")
		require.NoError(t, err)

		features, err := deployment.ListEnvironmentFeatures(ctx, env.ID)
		require.NoError(t, err)

		assert.Empty(t, features)
	})
}
