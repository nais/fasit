package rollouts

import (
	"testing"
	"time"
)

func TestGroupDeployments(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 1, 1, 11, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 1, 13, 0, 0, 0, time.UTC)

	t.Run("multiple features are grouped separately", func(t *testing.T) {
		items := []Summary{
			{FeatureName: "alpha", Target: "prod", createdAt: t1},
			{FeatureName: "beta", Target: "prod", createdAt: t2},
		}
		groups := groupDeployments(items)
		if len(groups) != 2 {
			t.Fatalf("expected 2 groups, got %d", len(groups))
		}
		names := map[string]bool{groups[0].FeatureName: true, groups[1].FeatureName: true}
		if !names["alpha"] || !names["beta"] {
			t.Errorf("expected groups for alpha and beta, got %v", names)
		}
	})

	t.Run("duplicate targets within a feature keep only the latest", func(t *testing.T) {
		items := []Summary{
			{FeatureName: "alpha", Target: "prod", Version: "v1", createdAt: t0},
			{FeatureName: "alpha", Target: "prod", Version: "v2", createdAt: t1},
		}
		groups := groupDeployments(items)
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		if len(groups[0].Targets) != 1 {
			t.Fatalf("expected 1 target after dedup, got %d", len(groups[0].Targets))
		}
		if groups[0].Targets[0].Version != "v2" {
			t.Errorf("expected latest version v2, got %s", groups[0].Targets[0].Version)
		}
	})

	t.Run("groups sorted by most-recent-first", func(t *testing.T) {
		items := []Summary{
			{FeatureName: "older", Target: "prod", createdAt: t1},
			{FeatureName: "newer", Target: "prod", createdAt: t3},
			{FeatureName: "middle", Target: "prod", createdAt: t2},
		}
		groups := groupDeployments(items)
		if len(groups) != 3 {
			t.Fatalf("expected 3 groups, got %d", len(groups))
		}
		if groups[0].FeatureName != "newer" || groups[1].FeatureName != "middle" || groups[2].FeatureName != "older" {
			t.Errorf("groups not sorted most-recent-first: got %s, %s, %s",
				groups[0].FeatureName, groups[1].FeatureName, groups[2].FeatureName)
		}
	})

	t.Run("rows within a group sorted by most-recent-first", func(t *testing.T) {
		items := []Summary{
			{FeatureName: "alpha", Target: "dev", Version: "v1", createdAt: t0},
			{FeatureName: "alpha", Target: "staging", Version: "v2", createdAt: t2},
			{FeatureName: "alpha", Target: "prod", Version: "v3", createdAt: t1},
		}
		groups := groupDeployments(items)
		if len(groups) != 1 {
			t.Fatalf("expected 1 group, got %d", len(groups))
		}
		targets := groups[0].Targets
		if len(targets) != 3 {
			t.Fatalf("expected 3 targets, got %d", len(targets))
		}
		if targets[0].Version != "v2" || targets[1].Version != "v3" || targets[2].Version != "v1" {
			t.Errorf("targets not sorted most-recent-first: got %s, %s, %s",
				targets[0].Version, targets[1].Version, targets[2].Version)
		}
	})
}
