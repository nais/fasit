package upgrader

import (
	"context"
	"github.com/nais/fasit/pkg/upgrader/mocks"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestClient_GetReleaseChannel(t *testing.T) {
	ctx := context.Background()
	mock := mocks.NewUpgrader(t)
	mock.EXPECT().GetReleaseChannel(ctx, "projectId", "clusterName").Return("STABLE", nil)
	channel, err := mock.GetReleaseChannel(ctx, "projectId", "clusterName")
	assert.NoError(t, err)
	assert.Equal(t, "STABLE", channel)
}
