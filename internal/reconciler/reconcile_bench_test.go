//go:build integration_test && reconciler_bench

package reconciler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/reconciler"
)

// TestReconcileWorkerPoolScaling measures compute phase duration.
func TestReconcileWorkerPoolScaling(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	const (
		numFeatures = 100
		numTenants  = 10
	)
	envKinds := []string{"dev", "staging", "prod"}

	h := newReconcileTest(ctx, t, container, dsn)

	var envs []tenantEnv
	for ti := range numTenants {
		tenant := fmt.Sprintf("tenant-%02d", ti)
		for _, kind := range envKinds {
			envs = append(envs, tenantEnv{tenant, kind, environment.Labels{"kind": "tenant"}})
		}
	}
	h.createEnvs(envs...)

	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		values := feature.Values{
			"setting_a":     {Config: &feature.Config{Type: feature.ConfigTypeString}, DisplayName: "Setting A"},
			"setting_b":     {Config: &feature.Config{Type: feature.ConfigTypeString}, DisplayName: "Setting B"},
			"computed_name": {Computed: &feature.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
			"computed_cfg":  {Computed: &feature.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
		}
		defaults := map[string]any{
			"setting_a": fmt.Sprintf("default-a-%d", fi),
			"setting_b": fmt.Sprintf("default-b-%d", fi),
		}
		h.createAssignmentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
	}

	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		for _, key := range []string{"setting_a", "setting_b"} {
			b, _ := json.Marshal(fmt.Sprintf("global-%s-%d", key, fi))
			if _, err := feature.ConfigGlobalCreate(h.ctx, feature.NewConfiguration{
				Feature: name, Key: key, Value: b,
			}); err != nil {
				t.Fatalf("create config: %v", err)
			}
		}
	}

	result, err := h.reconciler.ComputeDesiredState(h.ctx)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	deployCount := 0
	for _, d := range result.Decisions {
		if d.Action == reconciler.ActionDeploy {
			deployCount++
		}
	}

	t.Logf("fetch=%-10s  compute=%-10s  total=%-10s  decisions=%d  deploys=%d",
		result.FetchDur.Round(time.Millisecond),
		result.ComputeDur.Round(time.Millisecond),
		(result.FetchDur + result.ComputeDur).Round(time.Millisecond),
		len(result.Decisions),
		deployCount,
	)
}
