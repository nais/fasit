package cluster

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/environment/environmenttest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/sdk/metric"
)

func newAutoUpgrader(suite *testSuite) *AutoUpgrader {
	log := logrus.New().WithField("testSuite", "upgrade")
	meter := metric.NewMeterProvider().Meter("testSuite")
	return NewAutoUpgrader(suite.repoMock, log, suite.clusterMock, meter)
}

func Test_IsNewerPatchRelease(t *testing.T) {
	suite := newTestSuite(t)
	upgrader := newAutoUpgrader(suite)

	tests := []struct {
		name     string
		version1 string
		version2 string
		want     bool
	}{
		{
			name:     "Test newer version",
			version1: "1.28.10-gke.1075000",
			version2: "1.28.11-gke.1075000",
			want:     true,
		},
		{
			name:     "Test newer version",
			version1: "1.28.11-gke.1075000",
			version2: "1.28.11-gke.1075001",
			want:     true,
		},
		{
			name:     "Test newer version",
			version1: "1.28.11-gke.1075000",
			version2: "1.29.11-gke.1075001",
			want:     false,
		},
		{
			name:     "Test older version",
			version1: "1.28.11-gke.1075000",
			version2: "1.28.10-gke.1075000",
			want:     false,
		},
		{
			name:     "Test same version",
			version1: "1.28.10-gke.1075000",
			version2: "1.28.10-gke.1075000",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := upgrader.IsNewerPatchRelease(tt.version1, tt.version2); got != tt.want {
				t.Errorf("IsNewerVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcessEnvironment_TenantLookupCalledOnce(t *testing.T) {
	ctx := environmenttest.RegisterMock(context.Background(), t)
	suite := newTestSuite(t)
	upgrader := newAutoUpgrader(suite)

	suite.repoMock.EXPECT().EnvironmentValueGet(mock.Anything, suite.env.id, "project_id", false).Return(
		&model.EnvironmentValue{Key: "project_id", Value: []byte(`"test-project"`)}, nil,
	).Once()

	environmenttest.GetQuerier(ctx).EXPECT().GetTenant(mock.Anything, suite.env.tenantID).Return(
		environmentsql.Tenant{
			ID:               suite.env.tenantID,
			Name:             suite.env.name,
			Created:          pgtype.Timestamptz{Valid: true},
			LastModified:     pgtype.Timestamptz{Valid: true},
			UpgradeDelayDays: 0,
		}, nil,
	).Once()

	suite.clusterMock.EXPECT().GetCurrentControlPlaneVersion(mock.Anything, "test-project", suite.environment).Return("1.33.5-gke.100", nil).Once()
	suite.clusterMock.EXPECT().GetReleaseChannel(mock.Anything, "test-project", suite.environment).Return("REGULAR", nil).Once()
	suite.clusterMock.EXPECT().GetAvailableVersions(mock.Anything, "test-project", suite.environment, "REGULAR").Return([]string{"1.33.6-gke.200"}, nil).Once()

	suite.repoMock.EXPECT().ClusterUpgradeGet(mock.Anything, suite.env.tenantID, suite.env.id).Return(nil, nil).Once()
	suite.repoMock.EXPECT().ClusterUpgradeGetByVersion(mock.Anything, suite.env.tenantID, suite.env.id, "1.33.6-gke.200").Return(nil, nil).Once()
	suite.repoMock.EXPECT().CreateClusterUpgrade(mock.Anything, suite.env.tenantID, suite.env.id, "1.33.6-gke.200", mock.Anything).Return(
		&model.ClusterUpgradeStatus{ID: uuid.New(), Version: "1.33.6-gke.200", UpgradeStatus: model.UpgradeStatusCreated}, nil,
	).Once()

	processed, scheduled := upgrader.processEnvironment(ctx, suite.environment, upgrader.createEnvironmentLogger(suite.environment, nil))
	if !processed {
		t.Fatalf("expected environment to be processed")
	}
	if !scheduled {
		t.Fatalf("expected environment upgrade to be scheduled")
	}
}
