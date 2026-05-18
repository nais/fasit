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

func TestConfigCreate(t *testing.T) {
	tests := []struct {
		name        string
		input       model.NewConfiguration
		existingErr error
		existing    featuresql.ConfigurationsGlobal
		wantWrite   bool
		wantAudit   string // verb in metadata, empty = no audit
	}{
		{
			name: "ConfigCreate(global, new): creates and audits",
			input: model.NewConfiguration{
				Feature: "f1", Key: "my.key", Value: []byte(`"v"`),
			},
			existingErr: pgx.ErrNoRows,
			wantWrite:   true,
			wantAudit:   "create",
		},
		{
			name: "ConfigCreate(global, same value): no-op",
			input: model.NewConfiguration{
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
			fq.configGlobalUpdateOrCreateFunc = func(_ context.Context, arg featuresql.ConfigGlobalUpdateOrCreateParams) (featuresql.ConfigurationsGlobal, error) {
				wrote = true
				return featuresql.ConfigurationsGlobal{
					ID: uuid.New(), Feature: arg.Feature, Key: arg.Key, Value: arg.Value,
				}, nil
			}

			_, err := ConfigCreate(ctx, tc.input)
			if err != nil {
				t.Fatal(err)
			}

			if tc.wantWrite && !wrote {
				t.Error("expected write")
			}
			if !tc.wantWrite && wrote {
				t.Error("unexpected write")
			}
			if tc.wantAudit != "" && len(aq.Creates) == 0 {
				t.Fatal("expected audit")
			}
			if tc.wantAudit != "" {
				assertAuditMetadata(t, aq.Creates[0].Metadata, "verb", tc.wantAudit)
			}
			if tc.wantAudit == "" && len(aq.Creates) != 0 {
				t.Errorf("got %d audit calls, want 0", len(aq.Creates))
			}
		})
	}
}

func TestConfigCreate_Env(t *testing.T) {
	tests := []struct {
		name      string
		existing  *featuresql.ConfigurationsEnvironment
		secret    bool
		wantAudit map[string]any
	}{
		{
			name: "ConfigCreate(env, update secret): redacted audit",
			existing: &featuresql.ConfigurationsEnvironment{
				ID: uuid.New(), Feature: "f1", Key: "k",
				Value: []byte(`"old"`), Secret: true,
			},
			secret: true,
			wantAudit: map[string]any{
				"verb":   "update",
				"secret": true,
				"before": "<redacted>",
				"after":  "<redacted>",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, fq, aq := newTestCtx(t)
			envID := uuid.New()

			fq.configEnvGetFunc = func(_ context.Context, _ featuresql.ConfigEnvGetParams) (featuresql.ConfigurationsEnvironment, error) {
				if tc.existing == nil {
					return featuresql.ConfigurationsEnvironment{}, pgx.ErrNoRows
				}
				return *tc.existing, nil
			}

			fq.configEnvUpdateOrCreateFunc = func(_ context.Context, arg featuresql.ConfigEnvUpdateOrCreateParams) (featuresql.ConfigurationsEnvironment, error) {
				return featuresql.ConfigurationsEnvironment{
					ID: uuid.New(), EnvironmentID: envID,
					Feature: arg.Feature, Key: arg.Key, Value: arg.Value, Secret: arg.Secret,
				}, nil
			}

			_, err := ConfigCreate(ctx, model.NewConfiguration{
				EnvironmentID: &envID,
				Feature:       "f1",
				Key:           "k",
				Value:         []byte(`"new"`),
				Secret:        tc.secret,
			})
			if err != nil {
				t.Fatal(err)
			}

			if len(aq.Creates) != 1 {
				t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
			}
			for k, want := range tc.wantAudit {
				assertAuditMetadataAny(t, aq.Creates[0].Metadata, k, want)
			}
		})
	}
}

func TestConfigUpdate(t *testing.T) {
	tests := []struct {
		name      string
		existing  featuresql.ConfigurationsGlobal
		newValue  []byte
		wantWrite bool
		wantAudit string
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
			wantAudit: "update",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, fq, aq := newTestCtx(t)

			fq.configGetByIDFunc = func(_ context.Context, _ uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
				return tc.existing, nil
			}

			var wrote bool
			fq.configUpdateFunc = func(_ context.Context, arg featuresql.ConfigUpdateParams) (featuresql.ConfigurationsGlobal, error) {
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
			if tc.wantAudit != "" {
				if len(aq.Creates) != 1 {
					t.Fatalf("got %d audit calls, want 1", len(aq.Creates))
				}
				assertAuditMetadata(t, aq.Creates[0].Metadata, "verb", tc.wantAudit)
			}
		})
	}
}

func TestConfigDelete(t *testing.T) {
	ctx, fq, aq := newTestCtx(t)
	id := uuid.New()

	fq.configGetByIDFunc = func(_ context.Context, got uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
		if got != id {
			t.Fatalf("ConfigGetByID(%v), want %v", got, id)
		}
		return featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: []byte(`"v"`)}, nil
	}

	var deleted bool
	fq.configDeleteFunc = func(_ context.Context, got uuid.UUID) error {
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
	assertAuditMetadata(t, aq.Creates[0].Metadata, "verb", "delete")
	assertAuditMetadata(t, aq.Creates[0].Metadata, "before", "v")
}

func assertAuditMetadataAny(t *testing.T, raw []byte, key string, want any) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal audit metadata: %v", err)
	}
	got := m[key]
	wantJSON, _ := json.Marshal(want)
	gotJSON, _ := json.Marshal(got)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("audit metadata[%q] = %s, want %s", key, gotJSON, wantJSON)
	}
}
