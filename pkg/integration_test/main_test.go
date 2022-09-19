package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRollout_integration(t *testing.T) {
	ctx := context.Background()
	mgr := &feature.Manager{}

	mgr.Features = []feature.Feature{
		{
			Name:    "feature",
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

	db.ExecContext(ctx, `INSERT INTO tenants (id, name, ci) VALUES ($1, 'tenant1', true)`, tenantID)
	db.ExecContext(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, 'env1', 'tenant', true)`, tenantEnvID, tenantID)
	db.ExecContext(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, 'env1', 'management', true)`, managementEnvID, tenantID)

	rollout, _ := rollout.New(ctx, mgr, repo)
	rollout.AllowAll = true

	sqlWorker := workers.NewRollout(repo, log)

	go func() {
		err := sqlWorker.Listen(ctx)
		if err != nil {
			t.Fatalf("error running sql worker: %v", err)
		}
	}()

	w := httptest.NewRecorder()
	body := []byte(`{"imageTag": "sitronterte"}`)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("feature", "feature")
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

	time.Sleep(10 * time.Millisecond)

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
		Feature: "feature",
		Changeset: &model.RolloutChangeset{
			New: map[string]json.RawMessage{
				"imageTag": json.RawMessage(`"sitronterte"`),
			},
		},
	}

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.Rollout{}, "Created", "LastModified"),
	}

	if !cmp.Equal(want, pendingRollout, cmpOpts...) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, pendingRollout, cmpOpts...))
	}
}
