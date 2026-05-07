package feature

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsql"
	auditmocks "github.com/nais/fasit/internal/audit/auditsql/mocks"
	"github.com/nais/fasit/internal/feature/featuresql"
	featuremocks "github.com/nais/fasit/internal/feature/featuresql/mocks"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/mock"
)

func setupTestCtx(t *testing.T) (context.Context, *featuremocks.Querier) {
	log, _ := test.NewNullLogger()
	fq := featuremocks.NewQuerier(t)
	aq := auditmocks.NewQuerier(t)
	ctx := context.WithValue(context.Background(), QuerierKey, featuresql.Querier(fq))
	ctx = audit.RegisterTestDeps(ctx, aq, log)
	return ctx, fq
}

func expectAudit(t *testing.T, ctx context.Context, wantDescPrefix string, wantMeta map[string]any) {
	t.Helper()
	q := ctx.Value(audit.QuerierKey).(*auditmocks.Querier)
	q.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(func(p auditsql.AuditCreateParams) bool {
		if !startsWith(p.Description, wantDescPrefix) {
			return false
		}
		var got map[string]any
		if err := json.Unmarshal(p.Metadata, &got); err != nil {
			return false
		}
		for k, v := range wantMeta {
			if got[k] != v {
				return false
			}
		}
		return true
	})).Return(nil).Once()
}

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestFeatureStatesEnable_HappyPath(t *testing.T) {
	ctx, q := setupTestCtx(t)
	envID := uuid.New()
	feat := &model.Feature{Name: "f1"}

	q.EXPECT().FeatureStateGet(mock.Anything, mock.Anything).Return(featuresql.FeatureState{}, pgx.ErrNoRows).Once()
	q.EXPECT().FeatureStateCreateOrUpdate(mock.Anything, mock.MatchedBy(func(p featuresql.FeatureStateCreateOrUpdateParams) bool {
		return p.EnvironmentID == envID && p.Feature == "f1" && p.Enabled
	})).Return(featuresql.FeatureState{EnvironmentID: envID, Feature: "f1", Enabled: true}, nil).Once()

	expectAudit(t, ctx, "enabled", map[string]any{
		"verb":    "enable",
		"feature": "f1",
		"envId":   envID.String(),
	})

	if _, err := FeatureStatesEnable(ctx, envID, feat); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureStatesEnable_NoOpAlreadyEnabled(t *testing.T) {
	ctx, q := setupTestCtx(t)
	envID := uuid.New()
	feat := &model.Feature{Name: "f1"}

	q.EXPECT().FeatureStateGet(mock.Anything, mock.Anything).Return(featuresql.FeatureState{
		EnvironmentID: envID, Feature: "f1", Enabled: true,
	}, nil).Once()

	if _, err := FeatureStatesEnable(ctx, envID, feat); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureStatesDisable_HappyPath(t *testing.T) {
	ctx, q := setupTestCtx(t)
	envID := uuid.New()
	feat := &model.Feature{Name: "f1"}

	q.EXPECT().FeatureStateGet(mock.Anything, mock.Anything).Return(featuresql.FeatureState{
		EnvironmentID: envID, Feature: "f1", Enabled: true,
	}, nil).Once()
	q.EXPECT().FeatureStateCreateOrUpdate(mock.Anything, mock.MatchedBy(func(p featuresql.FeatureStateCreateOrUpdateParams) bool {
		return !p.Enabled && p.Feature == "f1"
	})).Return(featuresql.FeatureState{EnvironmentID: envID, Feature: "f1", Enabled: false}, nil).Once()

	expectAudit(t, ctx, "disabled: broken", map[string]any{
		"verb":    "disable",
		"feature": "f1",
		"envId":   envID.String(),
		"reason":  "broken",
	})

	if _, err := FeatureStatesDisable(ctx, envID, feat, "  broken  "); err != nil {
		t.Fatal(err)
	}
}

func TestFeatureStatesDisable_RequiresReason(t *testing.T) {
	ctx, q := setupTestCtx(t)
	envID := uuid.New()
	feat := &model.Feature{Name: "f1"}

	q.EXPECT().FeatureStateGet(mock.Anything, mock.Anything).Return(featuresql.FeatureState{
		EnvironmentID: envID, Feature: "f1", Enabled: true,
	}, nil).Once()

	if _, err := FeatureStatesDisable(ctx, envID, feat, "   "); err == nil {
		t.Fatal("expected error for empty reason")
	}
}

func TestFeatureStatesDisable_NoOpAlreadyDisabled(t *testing.T) {
	ctx, q := setupTestCtx(t)
	envID := uuid.New()
	feat := &model.Feature{Name: "f1"}

	q.EXPECT().FeatureStateGet(mock.Anything, mock.Anything).Return(featuresql.FeatureState{
		EnvironmentID: envID, Feature: "f1", Enabled: false,
	}, nil).Once()

	if _, err := FeatureStatesDisable(ctx, envID, feat, ""); err != nil {
		t.Fatal(err)
	}
}
