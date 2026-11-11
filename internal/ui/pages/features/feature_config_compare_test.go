//go:build integration_test

package features

import (
	"context"
	"encoding/json"
	"testing"

	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/testinfra"
)

func TestCompareKeyAcrossEnvsUsesAssignedVersion(t *testing.T) {
	ctx := context.Background()
	db := testinfra.Start(ctx, t)
	pool := db.Pool(ctx, t)
	ctx = featurepkg.Register(testinfra.Context(ctx, pool), pool)

	for _, name := range []string{"atil", "dev-nais"} {
		tenant, err := envpkg.CreateTenant(ctx, &envpkg.TenantCreate{Name: name})
		if err != nil {
			t.Fatalf("create tenant: %v", err)
		}
		if _, err := envpkg.Create(ctx, &envpkg.EnvironmentCreate{
			TenantID: tenant.ID,
			Name:     "dev",
			Kind:     envpkg.EnvironmentKindTenant,
			Labels:   envpkg.Labels{"kind": "tenant"},
		}); err != nil {
			t.Fatalf("create environment: %v", err)
		}
	}

	seeder := featureassignmenttest.NewSeeder()
	for _, version := range []string{"old", "new"} {
		target := envpkg.Labels{"kind": "tenant"}
		if version == "new" {
			target["tenant"] = "dev-nais"
		}
		switched := featurepkg.Value{Config: &featurepkg.Config{Type: featurepkg.ConfigTypeString}}
		if version == "new" {
			switched = featurepkg.Value{Computed: &featurepkg.Computed{Template: "new-computed"}}
		}
		seeder.AddAssignmentWithValues("pgrator", version, target, nil, featurepkg.Values{
			"image.tag": {Config: &featurepkg.Config{Type: featurepkg.ConfigTypeString}},
			"computed":  {Computed: &featurepkg.Computed{Template: version + "-{{ .Env.name }}"}},
			"switched":  switched,
		}, map[string]any{
			"image.tag": version,
			"switched":  "old-default",
		}, "")
	}
	originalDownloader := featureassignment.ChartDownloader
	featureassignment.ChartDownloader = seeder.ChartDownloader()
	t.Cleanup(func() { featureassignment.ChartDownloader = originalDownloader })
	if _, err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seed assignments: %v", err)
	}

	feat, err := featurepkg.FeatureByNameVersion(ctx, "pgrator", "new")
	if err != nil {
		t.Fatalf("load latest feature: %v", err)
	}
	envs, err := featureEnvironments(ctx, feat)
	if err != nil {
		t.Fatalf("load environments: %v", err)
	}
	if len(envs) != 2 {
		t.Fatalf("got %d environments, want 2", len(envs))
	}

	for _, tt := range []struct {
		name       string
		key        string
		wantValues []string
		wantSource []string
	}{
		{
			name:       "chart defaults",
			key:        "image.tag",
			wantValues: []string{"old", "new"},
			wantSource: []string{"helm value", "helm value"},
		},
		{
			name:       "computed templates",
			key:        "computed",
			wantValues: []string{"old-dev", "new-dev"},
			wantSource: []string{"mapping", "mapping"},
		},
		{
			name:       "value definition changes between versions",
			key:        "switched",
			wantValues: []string{"old-default", "new-computed"},
			wantSource: []string{"helm value", "mapping"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rows, err := compareKeyAcrossEnvs(ctx, feat, tt.key, envs)
			if err != nil {
				t.Fatalf("compare: %v", err)
			}
			if len(rows) != len(tt.wantValues) {
				t.Fatalf("got %d rows, want %d", len(rows), len(tt.wantValues))
			}
			for i, row := range rows {
				if row.Value != tt.wantValues[i] || row.Source != tt.wantSource[i] {
					t.Errorf("%s/%s: got (%q, %q), want (%q, %q)",
						row.TenantName, row.EnvName, row.Value, row.Source, tt.wantValues[i], tt.wantSource[i])
				}
			}
		})
	}

	t.Run("configuration overrides still take precedence", func(t *testing.T) {
		if _, err := featurepkg.CreateGlobalConfig(ctx, featurepkg.NewConfiguration{
			Feature: "pgrator",
			Key:     "image.tag",
			Value:   json.RawMessage(`"global"`),
		}); err != nil {
			t.Fatalf("create global config: %v", err)
		}
		if _, err := featurepkg.CreateEnvConfig(ctx, featurepkg.NewConfiguration{
			Feature:       "pgrator",
			EnvironmentID: &envs[0].ID,
			Key:           "image.tag",
			Value:         json.RawMessage(`"env"`),
		}); err != nil {
			t.Fatalf("create env config: %v", err)
		}
		rows, err := compareKeyAcrossEnvs(ctx, feat, "image.tag", envs)
		if err != nil {
			t.Fatalf("compare: %v", err)
		}
		if len(rows) != 2 {
			t.Fatalf("got %d rows, want 2", len(rows))
		}
		if rows[0].Value != "env" || rows[0].Source != "env config" {
			t.Errorf("environment override: got (%q, %q)", rows[0].Value, rows[0].Source)
		}
		if rows[1].Value != "global" || rows[1].Source != "global config" {
			t.Errorf("global override: got (%q, %q)", rows[1].Value, rows[1].Source)
		}
	})
}
