package deployments

import (
	"testing"
	"time"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/graph/model"
)

func TestLatestPerChartAndTarget(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)

	makeDep := func(chart, version string, labels map[string]string, created time.Time) *deployment.Deployment {
		return &deployment.Deployment{
			Feature:      &model.Feature{Chart: chart, Version: version},
			TargetLabels: labels,
			Created:      created,
		}
	}

	t.Run("keeps latest per chart+target", func(t *testing.T) {
		deps := []*deployment.Deployment{
			makeDep("oci://foo", "v1", map[string]string{"env": "prod"}, t0),
			makeDep("oci://foo", "v2", map[string]string{"env": "prod"}, t1),
			makeDep("oci://foo", "v3", map[string]string{"env": "prod"}, t2),
		}
		result := latestPerChartAndTarget(deps)
		if len(result) != 1 {
			t.Fatalf("expected 1 deployment, got %d", len(result))
		}
		if result[0].Feature.Version != "v3" {
			t.Errorf("expected v3, got %s", result[0].Feature.Version)
		}
	})

	t.Run("different targets kept separately", func(t *testing.T) {
		deps := []*deployment.Deployment{
			makeDep("oci://foo", "v1", map[string]string{"env": "prod"}, t0),
			makeDep("oci://foo", "v2", map[string]string{"env": "dev"}, t1),
		}
		result := latestPerChartAndTarget(deps)
		if len(result) != 2 {
			t.Fatalf("expected 2 deployments, got %d", len(result))
		}
	})

	t.Run("different charts kept separately", func(t *testing.T) {
		deps := []*deployment.Deployment{
			makeDep("oci://foo", "v1", map[string]string{"env": "prod"}, t0),
			makeDep("oci://bar", "v1", map[string]string{"env": "prod"}, t1),
		}
		result := latestPerChartAndTarget(deps)
		if len(result) != 2 {
			t.Fatalf("expected 2 deployments, got %d", len(result))
		}
	})

	t.Run("empty targets treated as same group", func(t *testing.T) {
		deps := []*deployment.Deployment{
			makeDep("oci://foo", "v1", nil, t0),
			makeDep("oci://foo", "v2", nil, t2),
		}
		result := latestPerChartAndTarget(deps)
		if len(result) != 1 {
			t.Fatalf("expected 1 deployment, got %d", len(result))
		}
		if result[0].Feature.Version != "v2" {
			t.Errorf("expected v2, got %s", result[0].Feature.Version)
		}
	})
}
