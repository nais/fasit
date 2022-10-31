//go:build integration_test_disable

package integration_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
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
			ConfigHash: "8837e7108dd58fd3b3e6ddf2badf1f38ac71eacb5077628b4222110aca2cde5b",
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

func TestRollout_tenant_and_management_success(t *testing.T) {
	const (
		newTag = "new-tag"
		oldTag = "old-tag"
	)
	ctx := context.Background()
	feat := feature.Feature{
		Name:    "feature4",
		Chart:   "oci://chart",
		Version: "10",
		EnvironmentKinds: []model.EnvironmentKind{
			model.EnvironmentKindTenant,
			model.EnvironmentKindManagement,
		},
		Config: feature.Config{
			"imageTag": feature.ConfigType{
				Type: "string",
			},
		},
	}

	tctx, close := NewTestContext(t, []feature.Feature{feat}, "env1", model.EnvironmentKindTenant)
	defer close()

	mgmID := uuid.New()

	_, err := tctx.DB.Exec(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, $3, $4, true)`, mgmID, tctx.TenantID, "management", model.EnvironmentKindManagement)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tctx.Repo.ConfigCreate(ctx, model.NewConfiguration{
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
	if err := tctx.Repo.HealthStatusCreateOrUpdate(ctx, mgmID, &message.Health{ReportedAt: time.Now()}); err != nil {
		t.Fatalf("tctx.Repo.HealthStatusCreateOrUpdate(ctx, tctx.EnvTenantID, &message.Health{ReportedAt: time.Now()}) = %v, want nil", err)
	}
	_, err = tctx.Repo.FeatureStatesCreateOrUpdate(database.WithNow(ctx, func() time.Time {
		return time.Date(2020, time.September, 11, 9, 4, 5, 6, time.UTC)
	}), mgmID, &feat, true)
	if err != nil {
		t.Fatal(err)
	}

	tctx.StartListeners()
	time.Sleep(1 * time.Second)

	rolloutID := tctx.PostRollout(t, feat.Name, map[string]any{"imageTag": newTag})

	wantRollouts := []*model.Rollout{
		{
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
		},
		{
			RolloutSummaryID: rolloutID,
			EnvironmentKind:  model.EnvironmentKindManagement,
			Feature:          feat.Name,
			Status:           model.RolloutStatusPending,
			Changeset: &model.RolloutChangeset{
				New: map[string]json.RawMessage{
					"imageTag": json.RawMessage(strconv.Quote(newTag)),
				},
				Old: map[string]json.RawMessage{
					"imageTag": json.RawMessage("null"),
				},
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, wantRollouts, model.RolloutStatusPending); err != nil {
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

	// Because of how the mocks in this test work, we send deploy instructions to both environments on the same topic.
	wantInstructions := []message.DeployInstruction{
		{
			Name:       feat.Name,
			Version:    feat.Version,
			Chart:      feat.Chart,
			Repo:       "",
			ConfigHash: "8837e7108dd58fd3b3e6ddf2badf1f38ac71eacb5077628b4222110aca2cde5b",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []byte(strconv.Quote(newTag)), Status: pgtype.Present}},
			RolloutIDs: []uuid.UUID{rolloutIDs[0]},
		},
		{
			Name:       feat.Name,
			Version:    feat.Version,
			Chart:      feat.Chart,
			ConfigHash: "0aa0e2b946664607f5d614f78db00d5c3b8eae72e9a2eb538f1445c384c5b5d0",
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []uint8(`"new-tag"`), Status: 2}},
			RolloutIDs: []uuid.UUID{rolloutIDs[1]},
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusDeployed)

	for i := range wantRollouts {
		wantRollouts[i].Status = model.RolloutStatusDeployed
	}
	if err := tctx.VerifyRollout(rolloutID, wantRollouts, model.RolloutStatusDeployed); err != nil {
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
