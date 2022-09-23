//go:build integration_test

package integration_test

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
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
		ID:      rolloutID,
		Feature: feat.Name,
		Status:  model.RolloutStatusPending,
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(newTag)),
			},
			Old: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(oldTag)),
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, wantRollout); err != nil {
		t.Fatal(err)
	}

	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     &rolloutID,
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
			ConfigHash: "0adcc1736efa834f56a40505d89abab2ca1b65e6a439b54f09f83609a712d80c",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []byte(strconv.Quote(newTag)), Status: pgtype.Present}},
			RolloutIDs: []uuid.UUID{rolloutID},
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusDeployed)

	wantRollout.Status = model.RolloutStatusDeployed
	if err := tctx.VerifyRollout(rolloutID, wantRollout); err != nil {
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
		Name:    "feature",
		Chart:   "oci://chart",
		Version: "10",
		Config: feature.Config{
			"imageTag": feature.ConfigType{
				Type: "string",
			},
		},
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
		ID:      rolloutID,
		Feature: feat.Name,
		Status:  model.RolloutStatusPending,
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(newTag)),
			},
			Old: map[string]json.RawMessage{
				"imageTag": json.RawMessage("null"),
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, wantRollout); err != nil {
		t.Fatal(err)
	}

	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     &rolloutID,
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
			ConfigHash: "0adcc1736efa834f56a40505d89abab2ca1b65e6a439b54f09f83609a712d80c",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []uint8(`"new-tag"`), Status: pgtype.Present}},
			RolloutIDs: []uuid.UUID{rolloutID},
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusFailed)

	wantRollout.Status = model.RolloutStatusFailed
	if err := tctx.VerifyRollout(rolloutID, wantRollout); err != nil {
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
		Name:    "feature",
		Chart:   "oci://chart",
		Version: "10",
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
		ID:      rolloutID,
		Feature: feat.Name,
		Status:  model.RolloutStatusPending,
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(newTag)),
			},
			Old: map[string]json.RawMessage{
				"imageTag": json.RawMessage(strconv.Quote(oldTag)),
			},
		},
	}

	if err := tctx.VerifyRollout(rolloutID, wantRollout); err != nil {
		t.Fatal(err)
	}

	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tctx.EnvID,
			FeatureName:   feat.Name,
			RolloutID:     &rolloutID,
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
			ConfigHash: "0adcc1736efa834f56a40505d89abab2ca1b65e6a439b54f09f83609a712d80c",
			Timeout:    0,
			Values:     map[string]any{"imageTag": pgtype.JSONB{Bytes: []uint8(`"new-tag"`), Status: pgtype.Present}},
			RolloutIDs: []uuid.UUID{rolloutID},
		},
	}

	if err := tctx.VerifyDeployInstructions(wantInstructions); err != nil {
		t.Fatal(err)
	}

	tctx.Naisd.SendStatus(model.RolloutStatusFailed)

	wantRollout.Status = model.RolloutStatusFailed
	if err := tctx.VerifyRollout(rolloutID, wantRollout); err != nil {
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
