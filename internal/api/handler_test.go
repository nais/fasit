//go:build integration_test

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/api"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymenttest"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	metricsdk "go.opentelemetry.io/otel/sdk/metric"
)

// TODO: simplify test setup and generalize across tests
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

	pub := &publisher{}
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

	handler, err := api.NewHttpHandler(ctx, mgr.db.pool, logger)
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

		var getResp api.GetDeploymentResponse
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

func startPostgresql(ctx context.Context, t *testing.T) (container *postgres.PostgresContainer, dsn string, err error) {
	t.Helper()

	container, err = postgres.Run(
		ctx,
		"docker.io/postgres:16-alpine",
		postgres.WithDatabase("test"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	defer testcontainers.CleanupContainer(t, container)

	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	dsn, err = container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	logger, _ := test.NewNullLogger()
	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("Error connecting to database: %v", err)
	}
	pool.Close()

	if err = container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, dsn, nil
}

type TestMgr struct {
	t         *testing.T
	db        Db
	seeder    *deploymenttest.Seeder
	publisher *publisher
	log       logrus.FieldLogger
}

type publisher struct {
	msg []message.DeployInstruction
}

func setupTestMgr(
	ctx context.Context,
	t *testing.T,
	container *postgres.PostgresContainer,
	dsn string,
	log logrus.FieldLogger,
) *TestMgr {
	t.Helper()
	db := getDb(ctx, t, container, dsn, log)
	seeder := deploymenttest.NewSeeder()
	pub := &publisher{}
	return &TestMgr{
		t:         t,
		db:        db,
		seeder:    seeder,
		publisher: pub,
		log:       log,
	}
}

func getDb(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log logrus.FieldLogger) Db {
	t.Helper()

	pool, _, err := database.NewConnPool(ctx, dsn, log)
	if err != nil {
		t.Fatalf("Error connecting to database: %v", err)
	}

	t.Cleanup(func() {
		pool.Close()
		if err = container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	return Db{
		t:    t,
		pool: pool,
	}
}

// Intentional uppercase to avoid var clashes
type Db struct {
	t    *testing.T
	pool *pgxpool.Pool
}

func (p *publisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)

	status := model.RolloutStatusDeployed
	if strings.HasSuffix(msg.Name, "-pending") {
		status = model.RolloutStatusPending
	}
	return deployment.UpdateDeployInstructionStatus(ctx, msg.ID, status)
}

func (p *publisher) Stop() {}

func (d *Db) createEnv(ctx context.Context, tenant *model.Tenant, name string, labels environment.Labels) {
	d.t.Helper()

	if labels["kind"] == "" {
		labels["kind"] = "tenant"
	}
	env, err := environment.Create(ctx, &model.EnvironmentCreate{
		Name:     name,
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKind(labels["kind"]),
	})
	if err != nil {
		d.t.Fatalf("create environment: %v", err)
	}
	lbls := environment.Labels{}
	maps.Copy(lbls, labels)
	lbls["tenant"] = tenant.Name
	lbls["environment"] = env.Name
	err = environment.SetLabels(ctx, env.ID, lbls)
	if err != nil {
		d.t.Fatalf("set environment labels: %v", err)
	}

	err = naisdstatus.Set(ctx, env.ID, &message.Health{
		ReportedAt: time.Now(),
	})
	if err != nil {
		d.t.Fatalf("create health status: %v", err)
	}
}

func (d *Db) createTenantsAndEnvironments(ctx context.Context, tenantsAndEnvs map[string]environment.Labels) {
	d.t.Helper()

	tenants := make(map[string]*model.Tenant)
	for te, lbls := range tenantsAndEnvs {
		p := strings.Split(te, ":")
		tenantName, envName := p[0], p[1]

		_, exists := tenants[tenantName]
		if !exists {
			var err error
			tenant, err := environment.CreateTenant(ctx, &model.TenantCreate{
				Name: tenantName,
			})
			if err != nil {
				d.t.Fatalf("create tenant: %v", err)
			}

			tenants[tenantName] = tenant
		}

		d.createEnv(ctx, tenants[tenantName], envName, lbls)
	}
}
