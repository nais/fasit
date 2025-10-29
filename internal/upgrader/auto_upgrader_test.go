package upgrader

import (
	"testing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/sdk/metric"
)

func newAutoUpgrader(suite *testSuite) *AutoUpgrader {
	log := logrus.New().WithField("testSuite", "upgrade")
	meter := metric.NewMeterProvider().Meter("testSuite")
	return NewAutoUpgrader(suite.repoMock, log, suite.upgradeMock, meter)
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
