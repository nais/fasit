package feature

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

func TestConfigCreate(t *testing.T) {
	tests := []struct {
		name        string
		input       NewConfiguration
		existingErr error
		existing    featuresql.ConfigurationsGlobal
		wantWrite   bool
		wantAudit   bool
	}{
		{
			name: "ConfigCreate(global, new): creates and audits",
			input: NewConfiguration{
				Feature: "f1", Key: "my.key", Value: []byte(`"v"`),
			},
			existingErr: pgx.ErrNoRows,
			wantWrite:   true,
			wantAudit:   true,
		},
		{
			name: "ConfigCreate(global, same value): no-op",
			input: NewConfiguration{
				Feature: "f1", Key: "k", Value: []byte(`"v"`),
			},
			existing: featuresql.ConfigurationsGlobal{
				ID: uuid.New(), Feature: "f1", Key: "k", Value: []byte(`"v"`),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, fq, aq := newTestCtx(t)

			fq.configGlobalGetByKeyFunc = func(_ context.Context, _ featuresql.ConfigGlobalGetByKeyParams) (featuresql.ConfigurationsGlobal, error) {
				if tc.existingErr != nil {
					return featuresql.ConfigurationsGlobal{}, tc.existingErr
				}
				return tc.existing, nil
			}

			var wrote bool
			fq.configGlobalUpsertFunc = func(_ context.Context, arg featuresql.ConfigGlobalUpsertParams) (featuresql.ConfigurationsGlobal, error) {
				wrote = true
				return featuresql.ConfigurationsGlobal{
					ID: uuid.New(), Feature: arg.Feature, Key: arg.Key, Value: arg.Value,
				}, nil
			}

			_, err := ConfigGlobalCreate(ctx, tc.input)
			if err != nil {
				t.Fatal(err)
			}

			if tc.wantWrite && !wrote {
				t.Error("expected write")
			}
			if !tc.wantWrite && wrote {
				t.Error("unexpected write")
			}
			if tc.wantAudit && len(aq.Creates) == 0 {
				t.Fatal("expected audit")
			}
			if tc.wantAudit && aq.Creates[0].ObjectID != "f1/my.key" {
				t.Errorf("audit ObjectID = %q, want %q", aq.Creates[0].ObjectID, "f1/my.key")
			}
			if !tc.wantAudit && len(aq.Creates) != 0 {
				t.Errorf("got %d audit calls, want 0", len(aq.Creates))
			}
		})
	}
}

func TestConfigCreate_Env(t *testing.T) {
	ctx, fq, aq := newTestCtx(t)
	envID := uuid.New()

	fq.configEnvGetByKeyFunc = func(_ context.Context, _ featuresql.ConfigEnvGetByKeyParams) (featuresql.ConfigurationsEnvironment, error) {
		return featuresql.ConfigurationsEnvironment{
			ID: uuid.New(), Feature: "f1", Key: "k",
			Value: []byte(`"old"`), Secret: true,
		}, nil
	}

	fq.configEnvUpsertFunc = func(_ context.Context, arg featuresql.ConfigEnvUpsertParams) (featuresql.ConfigurationsEnvironment, error) {
		return featuresql.ConfigurationsEnvironment{
			ID: uuid.New(), EnvironmentID: envID,
			Feature: arg.Feature, Key: arg.Key, Value: arg.Value, Secret: arg.Secret,
		}, nil
	}

	_, err := ConfigEnvCreate(ctx, NewConfiguration{
		EnvironmentID: &envID,
		Feature:       "f1",
		Key:           "k",
		Value:         []byte(`"new"`),
		Secret:        true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(aq.Creates) != 1 {
		t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
	}
	got := aq.Creates[0]
	if got.ObjectID != "f1/k" {
		t.Errorf("ObjectID = %q, want %q", got.ObjectID, "f1/k")
	}
	if got.EnvironmentID == nil || *got.EnvironmentID != envID {
		t.Errorf("EnvironmentID = %v, want %v", got.EnvironmentID, envID)
	}
	if got.Action != "updated" {
		t.Errorf("Action = %q, want %q", got.Action, "updated")
	}
}

func TestConfigUpdate(t *testing.T) {
	tests := []struct {
		name      string
		existing  featuresql.ConfigurationsGlobal
		newValue  []byte
		wantWrite bool
		wantAudit bool
	}{
		{
			name:     "ConfigUpdate(same value): no-op",
			existing: featuresql.ConfigurationsGlobal{ID: uuid.New(), Feature: "f1", Key: "k", Value: []byte(`"x"`)},
			newValue: []byte(`"x"`),
		},
		{
			name:      "ConfigUpdate(different value): updates and audits",
			existing:  featuresql.ConfigurationsGlobal{ID: uuid.New(), Feature: "f1", Key: "k", Value: []byte(`"old"`)},
			newValue:  []byte(`"new"`),
			wantWrite: true,
			wantAudit: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, fq, aq := newTestCtx(t)

			fq.configGlobalGetByIDFunc = func(_ context.Context, _ uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
				return tc.existing, nil
			}

			var wrote bool
			fq.configGlobalUpdateFunc = func(_ context.Context, arg featuresql.ConfigGlobalUpdateParams) (featuresql.ConfigurationsGlobal, error) {
				wrote = true
				return featuresql.ConfigurationsGlobal{
					ID: tc.existing.ID, Feature: tc.existing.Feature, Key: tc.existing.Key, Value: arg.Value,
				}, nil
			}

			_, err := ConfigUpdate(ctx, tc.existing.ID, model.UpdateConfiguration{Value: tc.newValue})
			if err != nil {
				t.Fatal(err)
			}

			if tc.wantWrite != wrote {
				t.Errorf("wrote = %v, want %v", wrote, tc.wantWrite)
			}
			if tc.wantAudit {
				if len(aq.Creates) != 1 {
					t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
				}
				if aq.Creates[0].Action != "updated" {
					t.Errorf("Action = %q, want %q", aq.Creates[0].Action, "updated")
				}
				if aq.Creates[0].ObjectID != "f1/k" {
					t.Errorf("ObjectID = %q, want %q", aq.Creates[0].ObjectID, "f1/k")
				}
			}
		})
	}
}

func TestConfigDelete(t *testing.T) {
	ctx, fq, aq := newTestCtx(t)
	id := uuid.New()

	fq.configGlobalGetByIDFunc = func(_ context.Context, got uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
		if got != id {
			t.Fatalf("ConfigGetByID(%v), want %v", got, id)
		}
		return featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: []byte(`"v"`)}, nil
	}

	var deleted bool
	fq.configGlobalDeleteFunc = func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Fatalf("ConfigDelete(%v), want %v", got, id)
		}
		deleted = true
		return nil
	}

	if err := ConfigDelete(ctx, id); err != nil {
		t.Fatal(err)
	}

	if !deleted {
		t.Error("expected ConfigDelete to be called")
	}
	if len(aq.Creates) != 1 {
		t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
	}
	got := aq.Creates[0]
	if got.Action != "deleted" {
		t.Errorf("Action = %q, want %q", got.Action, "deleted")
	}
	if got.ObjectID != "f1/k" {
		t.Errorf("ObjectID = %q, want %q", got.ObjectID, "f1/k")
	}
}

// TestEnvConfig_PreservesIDAndCreated guards against a regression where merged
// rows lost their ID (and Created), causing UI URLs like /config/edit/<id> to
// collapse to the zero UUID for every row.
func TestEnvConfig_PreservesIDAndCreated(t *testing.T) {
	ctx, fq, _ := newTestCtx(t)

	globalID := uuid.New()
	envRowID := uuid.New()
	environmentID := uuid.New()
	globalCreated := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	envCreated := time.Date(2025, 2, 2, 0, 0, 0, 0, time.UTC)

	fq.configGlobalListByFeatureFunc = func(_ context.Context, _ string) ([]featuresql.ConfigurationsGlobal, error) {
		return []featuresql.ConfigurationsGlobal{
			{ID: globalID, Feature: "f", Key: "g-only", Value: []byte(`"g"`), Created: globalCreated},
		}, nil
	}
	fq.configEnvListByFeatureFunc = func(_ context.Context, _ featuresql.ConfigEnvListByFeatureParams) ([]featuresql.ConfigurationsEnvironment, error) {
		return []featuresql.ConfigurationsEnvironment{
			{ID: envRowID, Feature: "f", Key: "e-only", Value: []byte(`"e"`), Created: envCreated, EnvironmentID: environmentID},
		}, nil
	}

	got, err := EnvConfig(ctx, &Feature{Name: "f"}, environmentID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}

	byKey := map[string]*Configuration{}
	for _, c := range got {
		byKey[c.Key] = c
	}

	if g := byKey["g-only"]; g.ID != globalID || !g.Created.Equal(globalCreated) || g.Source != model.ConfigSourceGlobal {
		t.Errorf("global row = %+v, want ID=%v Created=%v Source=global", g, globalID, globalCreated)
	}
	if e := byKey["e-only"]; e.ID != envRowID || !e.Created.Equal(envCreated) || e.Source != model.ConfigSourceEnv {
		t.Errorf("env row = %+v, want ID=%v Created=%v Source=env", e, envRowID, envCreated)
	}
}
