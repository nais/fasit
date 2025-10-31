//go:build integration_test

package database

import (
	"context"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
)

func TestUpgradePriorityOrdering(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	// Create three test tenants with different upgrade delay days
	testTenant, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "test-tenant",
		Description: ptr.To("Test environment"),
	})
	require.NoError(t, err)

	prodTenant, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "prod-tenant",
		Description: ptr.To("Production environment"),
	})
	require.NoError(t, err)

	stagingTenant, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "staging-tenant",
		Description: ptr.To("Staging environment"),
	})
	require.NoError(t, err)

	// Set upgrade delay_days: test=0 (immediate), staging=0 (default), prod=2 (last)
	testTenant, err = repo.TenantSetUpgradeDelayDays(ctx, testTenant.ID, 0)
	require.NoError(t, err)
	prodTenant, err = repo.TenantSetUpgradeDelayDays(ctx, prodTenant.ID, 2)
	require.NoError(t, err)
	// staging keeps default delay_days 0

	// Verify delay_days values are set correctly
	assert.Equal(t, int32(0), testTenant.UpgradeDelayDays, "test tenant should have delay_days 0")
	assert.Equal(t, int32(2), prodTenant.UpgradeDelayDays, "prod tenant should have delay_days 2")

	// Get tenant again to verify persistence
	fetchedTest, err := repo.TenantGet(ctx, testTenant.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), fetchedTest.UpgradeDelayDays, "test tenant delay_days should persist")

	fetchedStaging, err := repo.TenantGet(ctx, stagingTenant.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), fetchedStaging.UpgradeDelayDays, "staging tenant should have default delay_days 0")

	fetchedProd, err := repo.TenantGet(ctx, prodTenant.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(2), fetchedProd.UpgradeDelayDays, "prod tenant delay_days should persist")
}

func TestEnvironmentUpgradePriorityOrdering(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	// Create a test tenant
	tenant, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "priority-test-tenant",
		Description: ptr.To("Test tenant for environment priorities"),
	})
	require.NoError(t, err)

	// Create three environments
	testEnv, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "test-env",
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKindTenant,
	})
	require.NoError(t, err)

	stagingEnv, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "staging-env",
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKindTenant,
	})
	require.NoError(t, err)

	prodEnv, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "prod-env",
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKindTenant,
	})
	require.NoError(t, err)

	// Set environment upgrade delay_days
	testEnv, err = repo.EnvironmentSetUpgradeDelayDays(ctx, testEnv.ID, 0)
	require.NoError(t, err)
	prodEnv, err = repo.EnvironmentSetUpgradeDelayDays(ctx, prodEnv.ID, 2)
	require.NoError(t, err)
	// staging keeps default delay_days 0

	// Verify delay_days values are set correctly
	assert.Equal(t, int32(0), testEnv.UpgradeDelayDays, "test env should have delay_days 0")
	assert.Equal(t, int32(2), prodEnv.UpgradeDelayDays, "prod env should have delay_days 2")

	// Get environment again to verify persistence
	fetchedTest, err := repo.EnvironmentGet(ctx, testEnv.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), fetchedTest.UpgradeDelayDays, "test env delay_days should persist")

	fetchedStaging, err := repo.EnvironmentGet(ctx, stagingEnv.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(0), fetchedStaging.UpgradeDelayDays, "staging env should have default delay_days 0")

	fetchedProd, err := repo.EnvironmentGet(ctx, prodEnv.ID)
	require.NoError(t, err)
	assert.Equal(t, int32(2), fetchedProd.UpgradeDelayDays, "prod env delay_days should persist")
}

func TestEnvironmentUpgradeConfigEnabled(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	// Create a test tenant and environment
	tenant, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "enabled-test-tenant",
		Description: ptr.To("Test tenant for enabled flag"),
	})
	require.NoError(t, err)

	env, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "test-env",
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKindTenant,
	})
	require.NoError(t, err)

	env, err = repo.EnvironmentAutoUpgradeSet(ctx, env.ID, true)
	require.NoError(t, err)

	// Set delay_days
	_, err = repo.EnvironmentSetUpgradeDelayDays(ctx, env.ID, 1)
	require.NoError(t, err)

	// Should appear in auto-upgrade list (auto_upgrade=true is sufficient)
	envs, err := repo.EnvironmentsGetByAutoUpgrade(ctx)
	require.NoError(t, err)
	found := false
	for _, e := range envs {
		if e.ID == env.ID {
			found = true
			assert.Equal(t, int32(1), e.UpgradeDelayDays, "environment should have delay_days 1")
			break
		}
	}
	assert.True(t, found, "environment with auto_upgrade=true should be in auto-upgrade list")
}
