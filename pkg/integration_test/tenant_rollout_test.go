//go:build integration_test

package integration_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

func TestRollout_tenant_success(t *testing.T) {
	const (
		newTag = "new-tag"
		oldTag = "old-tag"
	)
	ctx := context.Background()
	feat := feature.Feature{
		Name:    "feature",
		Chart:   "oci://chart",
		Version: "10",
		EnvironmentKinds: []model.EnvironmentKind{
			model.EnvironmentKindTenant,
		},
		Config: feature.Config{
			"imageTag": feature.ConfigType{
				Type: "string",
			},
		},
	}

	tctx, close := NewTestContext(t, []feature.Feature{feat}, "env1", model.EnvironmentKindTenant)
	defer close()

	_, err := tctx.Repo.ConfigCreate(ctx, model.NewConfiguration{
		EnvironmentID: &tctx.EnvID,
		Feature:       feat.Name,
		Key:           "imageTag",
		Value:         json.RawMessage(strconv.Quote(oldTag)),
	})
	if err != nil {
		t.Fatalf("tctx.Repo.ConfigCreate(ctx, config) = %v, want nil", err)
	}

	_, err = tctx.Repo.FeatureStatesCreateOrUpdate(database.WithNow(ctx, func() time.Time {
		return time.Date(2020, time.September, 11, 9, 4, 5, 6, time.UTC)
	}), tctx.EnvID, &feat, true)
	if err != nil {
		t.Fatal(err)
	}

	if err := tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvID, &message.Health{ReportedAt: time.Now()}); err != nil {
		t.Fatalf("tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvTenantID, &message.Health{ReportedAt: time.Now()}) = %v, want nil", err)
	}

	tctx.StartListeners()
	time.Sleep(1 * time.Second)

	rolloutID := tctx.PostRollout(t, feat.Name, map[string]any{"imageTag": newTag})

	wantRollout := &model.Rollout{
		RolloutSummaryID: rolloutID,
		EnvironmentKind:  model.EnvironmentKindTenant,
		Feature:          feat.Name,
		Status:           model.RolloutStatusPending,
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(newTag)),
			},
			Old: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(oldTag)),
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, []*model.Rollout{wantRollout}, model.RolloutStatusPending); err != nil {
		t.Fatal(err)
	}

	rolloutIDs := tctx.RolloutSummaryRolloutIDs(t, ctx, rolloutID)

	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     &rolloutIDs[0],
		},
	}
	if err := tctx.VerifyEnvConfiguration(feat.Name, wantConfiguration); err != nil {
		t.Fatal(err)
	}

	wantInstructions := []message.DeployInstruction{
		{
			Name:       feat.Name,
			Version:    feat.Version,
			Chart:      feat.Chart,
			Repo:       "",
			ConfigHash: "0aa0e2b946664607f5d614f78db00d5c3b8eae72e9a2eb538f1445c384c5b5d0",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []byte(strconv.Quote(newTag)), Status: pgtype.Present}},
			RolloutIDs: rolloutIDs,
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusDeployed)

	wantRollout.Status = model.RolloutStatusDeployed
	if err := tctx.VerifyRollout(rolloutID, []*model.Rollout{wantRollout}, wantRollout.Status); err != nil {
		t.Fatal(err)
	}

	wantGlobalConfiguration := []*model.GlobalConfiguration{
		{
			Key:         "imageTag",
			Value:       json.RawMessage(strconv.Quote(newTag)),
			Type:        "",
			DisplayName: "",
			FeatureName: feat.Name,
		},
	}

	if err := tctx.VerifyGlobalConfiguration(feat.Name, wantGlobalConfiguration); err != nil {
		t.Fatal(err)
	}
}

func TestRollout_tenant_without_existing_config_failure(t *testing.T) {
	const (
		newTag = "new-tag"
	)
	ctx := context.Background()
	feat := feature.Feature{
		Name:    "feature2",
		Chart:   "oci://chart",
		Version: "10",
		Config: feature.Config{
			"imageTag": feature.ConfigType{
				Type: "string",
			},
		},
		EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
	}

	tctx, close := NewTestContext(t, []feature.Feature{feat}, "env1", model.EnvironmentKindTenant)
	defer close()

	if err := tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvID, &message.Health{ReportedAt: time.Now()}); err != nil {
		t.Fatalf("tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvTenantID, &message.Health{ReportedAt: time.Now()}) = %v, want nil", err)
	}

	defer tctx.StartListeners()()
	time.Sleep(1 * time.Second)

	rolloutID := tctx.PostRollout(t, feat.Name, map[string]any{"imageTag": newTag})

	wantRollout := &model.Rollout{
		Feature:          feat.Name,
		Status:           model.RolloutStatusPending,
		RolloutSummaryID: rolloutID,
		EnvironmentKind:  model.EnvironmentKindTenant,
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(newTag)),
			},
			Old: map[string]json.RawMessage{
				"imageTag": json.RawMessage("null"),
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, []*model.Rollout{wantRollout}, wantRollout.Status); err != nil {
		t.Fatal(err)
	}

	rolloutIDs := tctx.RolloutSummaryRolloutIDs(t, ctx, rolloutID)
	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     &rolloutIDs[0],
		},
	}
	if err := tctx.VerifyEnvConfiguration(feat.Name, wantConfiguration); err != nil {
		t.Fatal(err)
	}

	wantInstructions := []message.DeployInstruction{
		{
			Name:       feat.Name,
			Version:    feat.Version,
			Chart:      feat.Chart,
			Repo:       "",
			ConfigHash: "8837e7108dd58fd3b3e6ddf2badf1f38ac71eacb5077628b4222110aca2cde5b",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []uint8(`"new-tag"`), Status: pgtype.Present}},
			RolloutIDs: rolloutIDs,
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusFailed)

	wantRollout.Status = model.RolloutStatusFailed
	if err := tctx.VerifyRollout(rolloutID, []*model.Rollout{wantRollout}, wantRollout.Status); err != nil {
		t.Fatal(err)
	}

	wantConfiguration = []*model.EnvConfiguration{}
	if err := tctx.VerifyEnvConfiguration(feat.Name, wantConfiguration); err != nil {
		t.Fatal(err)
	}
}

func TestRollout_tenant_failure(t *testing.T) {
	const (
		newTag = "new-tag"
		oldTag = "old-tag"
	)
	ctx := context.Background()
	feat := feature.Feature{
		Name:             "feature3",
		Chart:            "oci://chart",
		Version:          "10",
		EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		Config: feature.Config{
			"imageTag": feature.ConfigType{
				Type: "string",
			},
		},
	}

	tctx, close := NewTestContext(t, []feature.Feature{feat}, "env1", model.EnvironmentKindTenant)
	defer close()

	_, err := tctx.Repo.ConfigCreate(ctx, model.NewConfiguration{
		EnvironmentID: &tctx.EnvID,
		Feature:       feat.Name,
		Key:           "imageTag",
		Value:         json.RawMessage(strconv.Quote(oldTag)),
	})
	if err != nil {
		t.Fatalf("tctx.Repo.ConfigCreate(ctx, config) = %v, want nil", err)
	}

	if err := tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvID, &message.Health{ReportedAt: time.Now()}); err != nil {
		t.Fatalf("tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvTenantID, &message.Health{ReportedAt: time.Now()}) = %v, want nil", err)
	}

	defer tctx.StartListeners()()
	time.Sleep(1 * time.Second)

	rolloutID := tctx.PostRollout(t, feat.Name, map[string]any{"imageTag": newTag})

	wantRollout := &model.Rollout{
		Feature:          feat.Name,
		Status:           model.RolloutStatusPending,
		RolloutSummaryID: rolloutID,
		EnvironmentKind:  model.EnvironmentKindTenant,
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(newTag)),
			},
			Old: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(oldTag)),
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, []*model.Rollout{wantRollout}, wantRollout.Status); err != nil {
		t.Fatal(err)
	}

	rolloutIDs := tctx.RolloutSummaryRolloutIDs(t, ctx, rolloutID)
	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     &rolloutIDs[0],
		},
	}
	if err := tctx.VerifyEnvConfiguration(feat.Name, wantConfiguration); err != nil {
		t.Fatal(err)
	}

	wantInstructions := []message.DeployInstruction{
		{
			Name:       feat.Name,
			Version:    feat.Version,
			Chart:      feat.Chart,
			Repo:       "",
			ConfigHash: "8837e7108dd58fd3b3e6ddf2badf1f38ac71eacb5077628b4222110aca2cde5b",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []uint8(`"new-tag"`), Status: pgtype.Present}},
			RolloutIDs: rolloutIDs,
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusFailed)

	wantRollout.Status = model.RolloutStatusFailed
	if err := tctx.VerifyRollout(rolloutID, []*model.Rollout{wantRollout}, wantRollout.Status); err != nil {
		t.Fatal(err)
	}

	wantConfiguration = []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(oldTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     nil,
		},
	}

	if err := tctx.VerifyEnvConfiguration(feat.Name, wantConfiguration); err != nil {
		t.Fatal(err)
	}
}
