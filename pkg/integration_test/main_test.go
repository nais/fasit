package integration_test

import (
	"bytes"
	"context"
	"github.com/go-chi/chi/v5"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/http/httptest"
	"testing"
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

	rollout, _ := rollout.New(ctx, mgr, repo)
	rollout.AllowAll = true

	w := httptest.NewRecorder()
	body := []byte(`{"imageTag": "sitronterte"}`)
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("feature", "feature")
	req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, chiCtx), "POST", "/rollout", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}

	rollout.Rollout(w, req)

	if w.Code != 201 {
		t.Log(w.Body.String())
		t.Fatalf("got %v, want 201", w.Code)
	}
}
