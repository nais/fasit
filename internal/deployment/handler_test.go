//go:build integration_test

package deployment_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

func TestCreateDeploymentHTTP(t *testing.T) {
	ctx := context.Background()
	logger, _ := test.NewNullLogger()

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	meterProvider := metricsdk.NewMeterProvider()
	httpMeter := meterProvider.Meter("handler-test-meter")

	mgr := setupTestMgr(ctx, t, container, dsn, logger)

	seeder := deploymenttest.NewSeeder()
	seeder.AddDeployment("my-feature", "1.0.0", environment.Labels{"kind": "tenant"})
	deployment.ChartDownloader = seeder.ChartDownloader()

	pub := &publisher{repo: mgr.db.repo}
	newPublisher := func(topicID string, log logrus.FieldLogger) deployment.Publisher {
		return pub
	}
	loadContext, err := contextloader.NewLoaderFunc(mgr.db.pool, newPublisher, httpMeter, logger)
	if err != nil {
		t.Fatalf("failed to get setup context: %v", err)
	}
	ctx = loadContext(ctx)

	mgr.db.createTenantsAndEnvironments(ctx, map[string]environment.Labels{
		"tenant1:dev": {"kind": "tenant"},
	})

	handler, err := deployment.NewHttpHandler(ctx, logger)
	if err != nil {
		t.Fatalf("create http handler: %v", err)
	}
	handler.AllowAll = true

	router := chi.NewMux()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = r.WithContext(loadContext(r.Context()))
			next.ServeHTTP(w, r)
		})
	})
	router.Post("/github/deployment", handler.CreateDeployment)
	router.Get("/github/deployment/{id}", handler.GetDeployment)

	t.Run("valid request returns 201 with id", func(t *testing.T) {
		body := `{
			"chart": "oci://my-feature",
			"version": "1.0.0",
			"target": {"kind": "tenant"}
		}`
		req := httptest.NewRequest(http.MethodPost, "/github/deployment", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusCreated, rr.Code)

		var resp map[string]string
		err := json.NewDecoder(rr.Body).Decode(&resp)
		require.NoError(t, err)

		_, err = uuid.Parse(resp["id"])
		assert.NoError(t, err, "response id should be a valid UUID")
	})

	t.Run("invalid JSON returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/github/deployment", bytes.NewBufferString(`{invalid`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("unknown chart returns 400", func(t *testing.T) {
		body := `{
			"chart": "oci://nonexistent",
			"version": "1.0.0",
			"target": {"kind": "tenant"}
		}`
		req := httptest.NewRequest(http.MethodPost, "/github/deployment", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("get deployment returns id and state", func(t *testing.T) {
		// Create a deployment first.
		body := `{
			"chart": "oci://my-feature",
			"version": "1.0.0",
			"target": {"kind": "tenant"}
		}`
		createReq := httptest.NewRequest(http.MethodPost, "/github/deployment", bytes.NewBufferString(body))
		createReq.Header.Set("Content-Type", "application/json")
		createRR := httptest.NewRecorder()
		router.ServeHTTP(createRR, createReq)
		require.Equal(t, http.StatusCreated, createRR.Code)

		var createResp map[string]string
		require.NoError(t, json.NewDecoder(createRR.Body).Decode(&createResp))

		// Get the deployment.
		getReq := httptest.NewRequest(http.MethodGet, "/github/deployment/"+createResp["id"], nil)
		getRR := httptest.NewRecorder()
		router.ServeHTTP(getRR, getReq)

		assert.Equal(t, http.StatusOK, getRR.Code)

		var getResp deployment.GetDeploymentResponse
		require.NoError(t, json.NewDecoder(getRR.Body).Decode(&getResp))
		assert.Equal(t, createResp["id"], getResp.ID.String())
	})

	t.Run("get nonexistent deployment returns 404", func(t *testing.T) {
		getReq := httptest.NewRequest(http.MethodGet, "/github/deployment/"+uuid.New().String(), nil)
		getRR := httptest.NewRecorder()
		router.ServeHTTP(getRR, getReq)

		assert.Equal(t, http.StatusNotFound, getRR.Code)
	})
}
