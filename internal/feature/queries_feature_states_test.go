package feature

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

func TestFeatureStatesEnable(t *testing.T) {
	tests := []struct {
		name      string
		existing  *featuresql.FeatureState // nil = not found
		wantWrite bool
		wantAudit string // empty = no audit expected
		wantNoOp  bool
	}{
		{
			name:      "Enable(new feature): creates enabled state",
			existing:  nil,
			wantWrite: true,
			wantAudit: "enable",
		},
		{
			name:     "Enable(already enabled): no-op",
			existing: &featuresql.FeatureState{Enabled: true},
			wantNoOp: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, fq, aq := newTestCtx(t)
			envID := uuid.New()
			feat := &model.Feature{Name: "f1"}

			fq.featureStateGetFunc = func(_ context.Context, arg featuresql.FeatureStateGetParams) (featuresql.FeatureState, error) {
				if arg.EnvironmentID != envID || arg.Feature != "f1" {
					t.Fatalf("unexpected FeatureStateGet args: %+v", arg)
				}
				if tc.existing == nil {
					return featuresql.FeatureState{}, pgx.ErrNoRows
				}
				return *tc.existing, nil
			}

			var wrote bool
			fq.featureStateCreateOrUpdateFunc = func(_ context.Context, arg featuresql.FeatureStateCreateOrUpdateParams) (featuresql.FeatureState, error) {
				wrote = true
				if !arg.Enabled {
					t.Error("expected Enabled=true")
				}
				return featuresql.FeatureState{EnvironmentID: envID, Feature: "f1", Enabled: true}, nil
			}

			got, err := FeatureStatesEnable(ctx, envID, feat)
			if err != nil {
				t.Fatal(err)
			}
			if got == nil {
				t.Fatal("got nil result")
			}

			if tc.wantNoOp && wrote {
				t.Error("expected no write, but FeatureStateCreateOrUpdate was called")
			}
			if tc.wantWrite && !wrote {
				t.Error("expected write, but FeatureStateCreateOrUpdate was not called")
			}
			if tc.wantAudit != "" {
				if len(aq.Creates) != 1 {
					t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
				}
				assertAuditMetadata(t, aq.Creates[0].Metadata, "verb", tc.wantAudit)
			}
			if tc.wantNoOp && len(aq.Creates) != 0 {
				t.Errorf("got %d audit calls, want 0", len(aq.Creates))
			}
		})
	}
}

func TestFeatureStatesDisable(t *testing.T) {
	tests := []struct {
		name      string
		existing  *featuresql.FeatureState
		reason    string
		wantErr   bool
		wantWrite bool
		wantAudit string
	}{
		{
			name:      "Disable(enabled, valid reason): disables and audits",
			existing:  &featuresql.FeatureState{Enabled: true},
			reason:    "  broken  ",
			wantWrite: true,
			wantAudit: "disable",
		},
		{
			name:     "Disable(enabled, blank reason): error",
			existing: &featuresql.FeatureState{Enabled: true},
			reason:   "   ",
			wantErr:  true,
		},
		{
			name:     "Disable(already disabled): no-op",
			existing: &featuresql.FeatureState{Enabled: false},
			reason:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, fq, aq := newTestCtx(t)
			envID := uuid.New()
			feat := &model.Feature{Name: "f1"}

			fq.featureStateGetFunc = func(_ context.Context, _ featuresql.FeatureStateGetParams) (featuresql.FeatureState, error) {
				if tc.existing == nil {
					return featuresql.FeatureState{}, pgx.ErrNoRows
				}
				return *tc.existing, nil
			}

			var wrote bool
			fq.featureStateCreateOrUpdateFunc = func(_ context.Context, arg featuresql.FeatureStateCreateOrUpdateParams) (featuresql.FeatureState, error) {
				wrote = true
				if arg.Enabled {
					t.Error("expected Enabled=false")
				}
				return featuresql.FeatureState{EnvironmentID: envID, Feature: "f1", Enabled: false}, nil
			}

			_, err := FeatureStatesDisable(ctx, envID, feat, tc.reason)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}

			if tc.wantWrite && !wrote {
				t.Error("expected write")
			}
			if !tc.wantWrite && wrote {
				t.Error("unexpected write")
			}
			if tc.wantAudit != "" {
				if len(aq.Creates) != 1 {
					t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
				}
				assertAuditMetadata(t, aq.Creates[0].Metadata, "verb", tc.wantAudit)
				assertAuditMetadata(t, aq.Creates[0].Metadata, "reason", "broken")
			}
		})
	}
}

func assertAuditMetadata(t *testing.T, raw []byte, key, want string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal audit metadata: %v", err)
	}
	got, _ := m[key].(string)
	if got != want {
		t.Errorf("audit metadata[%q] = %q, want %q", key, got, want)
	}
}
