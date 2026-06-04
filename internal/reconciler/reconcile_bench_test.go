//go:build integration_test && reconciler_bench

package reconciler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/reconciler"
)

// TestReconcileWorkerPoolScaling measures compute phase duration.
func TestReconcileWorkerPoolScaling(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)
	_ = container

	const (
		numFeatures = 100
		numTenants  = 10
	)
	envKinds := []string{"dev", "staging", "prod"}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	loadContext, err := contextloader.NewLoaderFunc(pool, logger)
	if err != nil {
		t.Fatalf("loader: %v", err)
	}
	ctx = loadContext(ctx)

	seeder := featureassignmenttest.NewSeeder()

	db := &reconcileDB{t: t, pool: pool}
	envsToCreate := map[string]environment.Labels{}
	for ti := range numTenants {
		tenant := fmt.Sprintf("tenant-%02d", ti)
		for _, kind := range envKinds {
			key := fmt.Sprintf("%s:%s", tenant, kind)
			envsToCreate[key] = environment.Labels{"kind": "tenant"}
		}
	}
	db.createTenantsAndEnvironments(ctx, envsToCreate)

	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		values := model.Values{
			"setting_a":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting A"},
			"setting_b":     {Config: &model.Config{Type: model.ConfigTypeString}, DisplayName: "Setting B"},
			"computed_name": {Computed: &model.Computed{Template: `"{{ .Env.name }}-{{ .Tenant.Name }}"`}},
			"computed_cfg":  {Computed: &model.Computed{Template: `"prefix-{{ .Configs.setting_a }}-suffix"`}},
		}
		defaults := map[string]any{
			"setting_a": fmt.Sprintf("default-a-%d", fi),
			"setting_b": fmt.Sprintf("default-b-%d", fi),
		}
		seeder.AddAssignmentWithValues(name, fmt.Sprintf("1.%d.0", fi), environment.Labels{"kind": "tenant"}, nil, values, defaults, fmt.Sprintf("Feature %d", fi))
	}
	featureassignment.ChartDownloader = seeder.ChartDownloader()
	if _, err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	for fi := range numFeatures {
		name := fmt.Sprintf("feature-%03d", fi)
		for _, key := range []string{"setting_a", "setting_b"} {
			b, _ := json.Marshal(fmt.Sprintf("global-%s-%d", key, fi))
			if _, err := feature.ConfigGlobalCreate(ctx, model.NewConfiguration{
				Feature: name, Key: key, Value: b,
			}); err != nil {
				t.Fatalf("create config: %v", err)
			}
		}
	}

	rec, err := reconciler.New(pool, meter, logger)
	if err != nil {
		t.Fatalf("create reconciler: %v", err)
	}

	result, err := rec.ComputeDesiredState(ctx)
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
		(result.FetchDur+result.ComputeDur).Round(time.Millisecond),
		len(result.Decisions),
		deployCount,
	)
}
