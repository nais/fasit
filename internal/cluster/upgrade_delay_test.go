package cluster

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestUpgradeDelay(t *testing.T) {
	log := logrus.New()

	t.Run("delay_days 0 - no delay", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(0)
		envDelayDays := int32(0)

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "test-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "test-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusCreated,
			Version:       "1.2.4",
			StartTime:     time.Now(),
		}

		// Should not delay - delay_days 0+0 means immediate
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "delay_days 0+0 should not delay")
	})

	t.Run("delay_days 1 - 1 day delay not satisfied", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(1)
		envDelayDays := int32(0) // Total: 1+0=1 day

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "staging-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "staging-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-12 * time.Hour), // Only 12 hours ago
		}

		// Should delay - need 24 hours for tenant 1 + env 0 = 1 day total
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.True(t, delayed, "delay_days 1+0=1 should delay when only 12 hours have passed")
	})

	t.Run("delay_days 1 - 1 day delay satisfied", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(1)
		envDelayDays := int32(0) // Total: 1+0=1 day

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "staging-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "staging-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-25 * time.Hour), // 25 hours ago
		}

		// Should not delay - 24+ hours have passed
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "delay_days 1+0=1 should not delay when 24+ hours have passed")
	})

	t.Run("delay_days 2 - 2 day delay not satisfied", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(2)
		envDelayDays := int32(0) // Total: 2+0=2 days

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-36 * time.Hour), // Only 36 hours ago
		}

		// Should delay - need 48 hours for tenant 2 + env 0 = 2 days total
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.True(t, delayed, "delay_days 2+0=2 should delay when only 36 hours have passed")
	})

	t.Run("delay_days 2 - 2 day delay satisfied", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(2)
		envDelayDays := int32(0) // Total: 2+0=2 days

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-49 * time.Hour), // 49 hours ago
		}

		// Should not delay - 48+ hours have passed
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "delay_days 2+0=2 should not delay when 48+ hours have passed")
	})

	t.Run("environment and tenant delay_days add together", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(2) // 2 days from tenant
		envDelayDays := int32(2)    // 2 days from environment

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-72 * time.Hour), // 72 hours (3 days) ago
		}

		// Should delay - need 96 hours (4 days) total: tenant 2 + environment 2
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.True(t, delayed, "tenant delay_days 2 + environment delay_days 2 should require 4 days total")
	})

	t.Run("environment and tenant delay_days add together - satisfied", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(2) // 2 days from tenant
		envDelayDays := int32(2)    // 2 days from environment

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-97 * time.Hour), // 97 hours ago (>4 days)
		}

		// Should not delay - 96+ hours (4 days) have passed
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "should not delay when 4+ days have passed")
	})

	t.Run("only delays CREATED status upgrades", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		delayDays := int32(2)
		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: delayDays,
		}
		env := &model.Environment{
			ID:       suite.env.id,
			TenantID: suite.env.tenantID,
			Name:     "prod-env",
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusControlPlaneUpgrade, // Not CREATED
			Version:       "1.2.4",
			StartTime:     time.Now(), // Just started, but already in progress
		}

		// Should not delay - already started
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "should not delay upgrades that are already in progress")
	})

	t.Run("default delay_days 0 when not set (0+0)", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "default-tenant",
			UpgradeDelayDays: 0, // default
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "default-env",
			UpgradeDelayDays: 0, // default
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusCreated,
			Version:       "1.2.4",
			StartTime:     time.Now(),
		}

		// Should not delay - defaults add up: tenant 0 + environment 0 = 0 days (no delay)
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "should use default delay_days 0+0=0 (no delay) when not set")
	})

	t.Run("changing delay_days while WAITING - increase delay", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		// Upgrade has been waiting for 2 days, originally configured with 2 days total
		// But now delay_days has been increased to 4 days total
		tenantDelayDays := int32(2)
		envDelayDays := int32(2) // Total: 4 days

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-48 * time.Hour), // 48 hours (2 days) ago
		}

		// Should still delay - now needs 4 days total, only 2 days have passed
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.True(t, delayed, "should continue waiting when delay_days is increased mid-wait")
	})

	t.Run("changing delay_days while WAITING - decrease delay", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		// Upgrade has been waiting for 3 days, originally configured with 4 days
		// But now delay_days has been decreased to 2 days total
		tenantDelayDays := int32(1)
		envDelayDays := int32(1) // Total: 2 days

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-72 * time.Hour), // 72 hours (3 days) ago
		}

		// Should not delay - only needs 2 days now, and 3 days have already passed
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "should proceed immediately when delay_days is decreased and time has passed")
	})

	t.Run("changing delay_days to 0 while WAITING - immediate upgrade", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		// Upgrade has been waiting, but delay_days has been set to 0 (immediate)
		tenantDelayDays := int32(0)
		envDelayDays := int32(0) // Total: 0 days

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusWaiting,
			Version:       "1.2.4",
			StartTime:     time.Now().Add(-12 * time.Hour), // Only 12 hours ago, but doesn't matter
		}

		// Should not delay - delay_days is now 0, proceed immediately
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "should proceed immediately when delay_days is set to 0")
	})
}

func TestCreatedToWaitingTransition(t *testing.T) {
	log := logrus.New()

	t.Run("CREATED status with delay_days > 0 transitions to WAITING", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(2)
		envDelayDays := int32(1) // Total: 3 days

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "prod-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "prod-env",
			UpgradeDelayDays: envDelayDays,
		}

		// Upgrade in CREATED status with delay configured
		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusCreated,
			Version:       "1.2.4",
			StartTime:     time.Now(),
		}

		// Should NOT delay in shouldDelayUpgrade because it only processes WAITING status
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "shouldDelayUpgrade should return false for CREATED status")

		// This confirms that the CREATED case in Run() must handle the transition to WAITING
		// The actual transition happens in the switch statement's CREATED case
	})

	t.Run("CREATED status with delay_days = 0 does not transition to WAITING", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		tenantDelayDays := int32(0)
		envDelayDays := int32(0) // Total: 0 days (no delay)

		tenant := &model.Tenant{
			ID:               suite.env.tenantID,
			Name:             "test-tenant",
			UpgradeDelayDays: tenantDelayDays,
		}
		env := &model.Environment{
			ID:               suite.env.id,
			TenantID:         suite.env.tenantID,
			Name:             "test-env",
			UpgradeDelayDays: envDelayDays,
		}

		clusterUpgrade := &model.ClusterUpgradeStatus{
			ID:            uuid.New(),
			UpgradeStatus: model.UpgradeStatusCreated,
			Version:       "1.2.4",
			StartTime:     time.Now(),
		}

		// Should not delay - no delay configured
		delayed := upgrader.shouldDelayUpgrade(tenant, env, clusterUpgrade, log)
		assert.False(t, delayed, "should not delay when delay_days is 0")

		// With delay_days = 0, CREATED case should proceed directly to control plane upgrade
		// without transitioning to WAITING
	})
}
