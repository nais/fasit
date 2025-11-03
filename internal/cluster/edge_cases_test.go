package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestErrorHandlingEdgeCases(t *testing.T) {
	t.Run("database error during tenant fetch", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return(nil, errors.New("database connection failed")).Maybe()

		err := upgrader.Run(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database connection failed")
	})

	t.Run("no cluster upgrade found returns nil", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)
		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return([]*model.Environment{{
			ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`"1234"`)}, nil).Maybe()

		// Mock ClusterUpgradeHistoryGet for the cleanup process - return empty list
		suite.repoMock.EXPECT().ClusterUpgradeHistoryGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(
			[]*model.ClusterUpgradeStatus{}, nil).Maybe()

		suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, pgx.ErrNoRows).Maybe()

		err := upgrader.Run(context.Background())
		assert.NoError(t, err) // Should complete successfully with no upgrade to process
	})

	t.Run("context cancellation during upgrade", func(t *testing.T) {
		suite := newTestSuite(t)
		upgrader := newUpgrade(suite)

		ctx, cancel := context.WithCancel(context.Background())

		suite.repoMock.EXPECT().TenantsGet(mock.Anything).Return([]*model.Tenant{{
			ID: suite.env.tenantID, Name: suite.env.name,
		}}, nil).Maybe()

		// Cancel context before environments fetch
		cancel()

		// This should handle cancellation gracefully
		suite.repoMock.EXPECT().EnvironmentsGet(mock.Anything, suite.env.tenantID).Return(nil, context.Canceled).Maybe()

		err := upgrader.Run(ctx)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, context.Canceled))
	})
}

func TestUpgradeEnvironmentEdgeCases(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newUpgrade(suite)

	t.Run("invalid project ID JSON", func(t *testing.T) {
		tenant := &model.Tenant{ID: suite.env.tenantID, Name: suite.env.name}
		env := &model.Environment{ID: suite.env.id, TenantID: suite.env.tenantID, Name: suite.env.name}

		suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, mock.Anything, "project_id", false).Return(
			&model.EnvironmentValue{Key: "project_id", Value: []byte(`invalid json`)}, nil).Maybe()

		err := upgrader.upgradeEnvironment(context.Background(), tenant, env)
		assert.Error(t, err)
	})
}
