package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	const (
		featureName = "feature"
		oldTag      = "existing"
		newTag      = "newtag"
	)

	mgr.Features = []feature.Feature{
		{
			Name:    featureName,
			Chart:   "oci://feature",
			Version: "69",
			Config: feature.Config{
				"imageTag": feature.ConfigType{
					Type: "string",
				},
			},
		},
	}

	db, dbConnString, close := dbtest.DockerSQLPool()
	defer close()
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
		featureName, json.RawMessage(strconv.Quote(oldTag)), tenantEnvID,
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

	w := httptest.NewRecorder()
	body := []byte(`{"imageTag": "` + newTag + `"}`)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("feature", featureName)
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

	pendingRollout, err := repo.RolloutGetByID(ctx, obj.Rollout)
	if err != nil {
		t.Fatalf("repo.RolloutGetByID(ctx, %v) = _, %v, want _, nil", obj.Rollout, err)
	}

	want := &model.Rollout{
		ID:      pendingRollout.ID,
		Feature: featureName,
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

	if !cmp.Equal(want, pendingRollout, cmpOpts...) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, pendingRollout, cmpOpts...))
	}

	confs, err := repo.ConfigGetForEnv(ctx, featureName, tenantEnvID)
	if err != nil {
		t.Fatalf("repo.ConfigGetForEnv(ctx, %v, %v) = _, %v, want _, nil", featureName, tenantEnvID, err)
	}

	cmpOpts = []cmp.Option{
		cmpopts.IgnoreFields(model.EnvConfiguration{}, "ID", "Created"),
	}

	want2 := []*model.EnvConfiguration{
		{
			Key:           "imageTag",
			Value:         json.RawMessage(strconv.Quote(newTag)),
			Type:          "",
			DisplayName:   "",
			EnvironmentID: tenantEnvID,
			FeatureName:   featureName,
		},
	}

	if !cmp.Equal(want2, confs, cmpOpts...) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want2, confs, cmpOpts...))
	}
}
