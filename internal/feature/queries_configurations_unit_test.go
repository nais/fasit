package feature

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsql"
	auditmocks "github.com/nais/fasit/internal/audit/auditsql/mocks"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func auditMatcher(t *testing.T, wantDesc string, wantMeta map[string]any) func(p auditsql.AuditCreateParams) bool {
	t.Helper()
	return func(p auditsql.AuditCreateParams) bool {
		if p.Description != wantDesc {
			return false
		}
		var got map[string]any
		if err := json.Unmarshal(p.Metadata, &got); err != nil {
			return false
		}
		for k, v := range wantMeta {
			gv, ok := got[k]
			if !ok {
				return false
			}
			gj, _ := json.Marshal(gv)
			vj, _ := json.Marshal(v)
			if string(gj) != string(vj) {
				return false
			}
		}
		return true
	}
}

func TestConfigCreate_Global_Creates(t *testing.T) {
	ctx, fq := setupTestCtx(t)
	aq := ctx.Value(audit.QuerierKey).(*auditmocks.Querier)
	id := uuid.New()

	fq.EXPECT().ConfigGlobalGetByKey(mock.Anything, mock.Anything).
		Return(featuresql.ConfigurationsGlobal{}, pgx.ErrNoRows).Once()
	fq.EXPECT().ConfigGlobalUpdateOrCreate(mock.Anything, mock.Anything).
		Return(featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "my.key", Value: []byte(`"v"`)}, nil).Once()

	aq.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(auditMatcher(t, "created config my.key", map[string]any{
		"verb":    "create",
		"feature": "f1",
		"key":     "my.key",
		"secret":  false,
		"after":   "v",
	}))).Return(nil).Once()

	_, err := ConfigCreate(ctx, model.NewConfiguration{Feature: "f1", Key: "my.key", Value: []byte(`"v"`)})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigCreate_Global_NoOpSameValue(t *testing.T) {
	ctx, fq := setupTestCtx(t)
	id := uuid.New()
	value := []byte(`"v"`)
	fq.EXPECT().ConfigGlobalGetByKey(mock.Anything, mock.Anything).
		Return(featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: value}, nil).Once()

	if _, err := ConfigCreate(ctx, model.NewConfiguration{Feature: "f1", Key: "k", Value: value}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigCreate_Env_UpdatesSecretRedacted(t *testing.T) {
	ctx, fq := setupTestCtx(t)
	aq := ctx.Value(audit.QuerierKey).(*auditmocks.Querier)
	envID := uuid.New()
	id := uuid.New()

	fq.EXPECT().ConfigEnvGet(mock.Anything, mock.Anything).
		Return(featuresql.ConfigurationsEnvironment{ID: id, EnvironmentID: envID, Feature: "f1", Key: "k", Value: []byte(`"old"`), Secret: true}, nil).Once()
	fq.EXPECT().ConfigEnvUpdateOrCreate(mock.Anything, mock.Anything).
		Return(featuresql.ConfigurationsEnvironment{ID: id, EnvironmentID: envID, Feature: "f1", Key: "k", Value: []byte(`"new"`), Secret: true}, nil).Once()

	aq.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(auditMatcher(t, "updated config k (secret)", map[string]any{
		"verb":    "update",
		"feature": "f1",
		"key":     "k",
		"secret":  true,
		"before":  "<redacted>",
		"after":   "<redacted>",
		"envId":   envID.String(),
	}))).Return(nil).Once()

	_, err := ConfigCreate(ctx, model.NewConfiguration{
		EnvironmentID: &envID,
		Feature:       "f1",
		Key:           "k",
		Value:         []byte(`"new"`),
		Secret:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestConfigUpdate_NoOp(t *testing.T) {
	ctx, fq := setupTestCtx(t)
	id := uuid.New()
	value := []byte(`"x"`)
	fq.EXPECT().ConfigGetByID(mock.Anything, id).
		Return(featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: value}, nil).Once()

	if _, err := ConfigUpdate(ctx, id, model.UpdateConfiguration{Value: value}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigUpdate_Updates(t *testing.T) {
	ctx, fq := setupTestCtx(t)
	aq := ctx.Value(audit.QuerierKey).(*auditmocks.Querier)
	id := uuid.New()

	fq.EXPECT().ConfigGetByID(mock.Anything, id).
		Return(featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: []byte(`"old"`)}, nil).Once()
	fq.EXPECT().ConfigUpdate(mock.Anything, mock.Anything).
		Return(featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: []byte(`"new"`)}, nil).Once()

	aq.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(auditMatcher(t, "updated config k", map[string]any{
		"verb":   "update",
		"key":    "k",
		"before": "old",
		"after":  "new",
	}))).Return(nil).Once()

	if _, err := ConfigUpdate(ctx, id, model.UpdateConfiguration{Value: []byte(`"new"`)}); err != nil {
		t.Fatal(err)
	}
}

func TestConfigDelete(t *testing.T) {
	ctx, fq := setupTestCtx(t)
	aq := ctx.Value(audit.QuerierKey).(*auditmocks.Querier)
	id := uuid.New()

	fq.EXPECT().ConfigGetByID(mock.Anything, id).
		Return(featuresql.ConfigurationsGlobal{ID: id, Feature: "f1", Key: "k", Value: []byte(`"v"`)}, nil).Once()
	fq.EXPECT().ConfigDelete(mock.Anything, id).Return(nil).Once()

	aq.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(auditMatcher(t, "deleted config k", map[string]any{
		"verb":   "delete",
		"key":    "k",
		"before": "v",
	}))).Return(nil).Once()

	if err := ConfigDelete(ctx, id); err != nil {
		t.Fatal(err)
	}
}
