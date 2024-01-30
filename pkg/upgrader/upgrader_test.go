package upgrader

import (
	"context"
	"testing"

	"github.com/nais/fasit/pkg/upgrader/mocks"
	"github.com/stretchr/testify/assert"
)

func TestClient_GetReleaseChannel(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewUpgrader(t)
	mock.EXPECT().GetReleaseChannel(ctx, "projectId", "clusterName").Return("STABLE", nil)
	channel, err := mock.GetReleaseChannel(ctx, "projectId", "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, "STABLE", channel)
}

func TestClient_GetRunningOperations(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewUpgrader(t)
	mock.EXPECT().GetRunningOperations(ctx, "projectId", "clusterName").Return(nil, nil)
	ops, err := mock.GetRunningOperations(ctx, "projectId", "clusterName")
	assert.NoError(t, err)
	assert.Nil(t, ops)
}
