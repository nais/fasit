package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/nais/fasit/pkg/message"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/nais/fasit/pkg/workers"
	"github.com/sirupsen/logrus"
)

func TestRollout_integration(t *testing.T) {
	ctx := context.Background()
	mgr := &feature.Manager{}
	featConfig := feature.Config{"imageTag": feature.ConfigType{
		Type: "string",
	}}
	feat := model.Feature{
		Name:             "feature",
		Chart:            "oci://chart",
		Version:          "1",
		Repo:             "",
		Source:           "",
		DependsOn:        nil,
		EnvironmentKinds: nil,
	}
	const (
		oldTag = "existing"
		newTag = "newtag"
	)

	mgr.Features = []feature.Feature{
		{
			Name:    feat.Name,
			Chart:   feat.Chart,
			Version: feat.Version,
			Config:  featConfig,
		},
	}

	db, dbConnString, close := dbtest.DockerSQLPool()
	defer close()
	logrus.StandardLogger().Level = logrus.DebugLevel
	log := logrus.NewEntry(logrus.StandardLogger())

	repo := database.New(db, dbConnString, log)
	err := database.Migrate(db, log)
	if err != nil {
		t.Fatalf("error migrating database: %v", err)
	}

	tenantEnvID := uuid.New()
	managementEnvID := uuid.New()
	tenantID := uuid.New()

	_, err = db.ExecContext(ctx, `INSERT INTO tenants (id, name, ci) VALUES ($1, 'tenant1', true)`, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, 'env1', 'tenant', true)`, tenantEnvID, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, 'env2', 'management', true)`, managementEnvID, tenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(
		ctx,
		`INSERT INTO configurations_environment (feature, key, value, environment_id) VALUES ($1, 'imageTag', $2, $3)`,
		feat.Name, json.RawMessage(strconv.Quote(oldTag)), tenantEnvID,
	)
	_, err = db.ExecContext(
		ctx,
		`INSERT INTO "feature_states" (environment_id, feature, enabled, enabled_at) VALUES ($1, $2, true, TIMESTAMP '2022-09-01 10:10:10')`,
		tenantEnvID, feat.Name,
	)

	_, err = db.ExecContext(
		ctx,
		`INSERT INTO "health_statuses" (environment_id, reported_at) VALUES ($1, NOW())`,
		tenantEnvID,
	)

	if err != nil {
		t.Fatal(err)
	}

	rollout, _ := rollout.New(ctx, mgr, repo)
	rollout.AllowAll = true

	sqlWorker := workers.NewRollout(repo, log)

	go func() {
		err := sqlWorker.Listen(ctx)
		if err != nil {
			panic(err)
		}
	}()

	publisher := &MockPublisher{}
	newPublisher := func(projectID string, topicID string, log *logrus.Entry) workers.Publisher {
		return publisher
	}

	reconciler := workers.NewReconciler(repo, mgr, newPublisher, "xxx", log)

	go func() {
		err = reconciler.Listen(ctx)
		if err != nil {
			panic(err)
		}
	}()

	w := httptest.NewRecorder()
	body := []byte(`{"imageTag": "` + newTag + `"}`)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("feature", feat.Name)
	req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, chiCtx), "POST", "/rollout", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)

	rollout.Rollout(w, req)

	if w.Code != 201 {
		t.Log(w.Body.String())
		t.Fatalf("got %v, want 201", w.Code)
	}

	time.Sleep(100 * time.Millisecond)

	obj := struct {
		Rollout uuid.UUID `json:"rollout"`
	}{}

	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		t.Fatalf("json.Unmarshal(jsonBody, v) = %v, want nil", err)
	}

	time.Sleep(100 * time.Millisecond)

	pendingRollout, err := repo.RolloutGetByID(ctx, obj.Rollout)
	if err != nil {
		t.Fatalf("repo.RolloutGetByID(ctx, %v) = _, %v, want _, nil", obj.Rollout, err)
	}

	wantRollout := &model.Rollout{
		ID:      pendingRollout.ID,
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

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.Rollout{}, "Created", "LastModified"),
	}

	if !cmp.Equal(wantRollout, pendingRollout, cmpOpts...) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(wantRollout, pendingRollout, cmpOpts...))
	}

	confs, err := repo.ConfigGetForEnv(ctx, feat.Name, tenantEnvID)
	if err != nil {
		t.Fatalf("repo.ConfigGetForEnv(ctx, %v, %v) = _, %v, want _, nil", feat.Name, tenantEnvID, err)
	}

	cmpOpts = []cmp.Option{
		cmpopts.IgnoreFields(model.EnvConfiguration{}, "ID", "Created"),
	}

	wantConfiguration := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tenantEnvID,
			FeatureName:   feat.Name,
		},
	}

	if !cmp.Equal(wantConfiguration, confs, cmpOpts...) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(wantConfiguration, confs, cmpOpts...))
	}

	wantInstructions := []message.DeployInstruction{
		{
			Name:       feat.Name,
			Version:    feat.Version,
			Chart:      feat.Chart,
			Repo:       "",
			ConfigHash: "a2a8c185faa8b051c0a519210ea83d2204695b740db0a5558fdd2c0bd0e2f298",
			Timeout:    0,
			Values:     map[string]any{"imageTag": json.RawMessage(`"newtag"`)},
		},
	}

	if !cmp.Equal(wantInstructions, publisher.deployInstruction, cmpOpts...) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(wantInstructions, publisher.deployInstruction, cmpOpts...))
	}
}

type MockPublisher struct {
	deployInstruction []message.DeployInstruction
}

func (m *MockPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	m.deployInstruction = append(m.deployInstruction, msg)
	return nil
}

func (m *MockPublisher) Stop() {}
