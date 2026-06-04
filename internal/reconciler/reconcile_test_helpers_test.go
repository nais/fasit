//go:build integration_test

package reconciler_test

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// reconcileFunc abstracts the reconciler behind one call.
type reconcileFunc func(ctx context.Context) error

// sqlPublisher records published messages and simulates naisd status updates
// via raw SQL so it works independently of the deployment context loader.
type sqlPublisher struct {
	pool *pgxpool.Pool
	msg  []message.DeployInstruction
}

func (p *sqlPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)
	status := model.RolloutStatusDeployed
	if strings.HasSuffix(msg.Name, "-pending") {
		status = model.RolloutStatusPending
	}
	_, err := p.pool.Exec(ctx, `UPDATE deploy_instructions SET status = $1 WHERE id = $2`, status.String(), msg.ID)
	return err
}

func (p *sqlPublisher) Stop() {}

type reconcileDB struct {
	t    *testing.T
	pool *pgxpool.Pool
}

func (d *reconcileDB) createEnv(ctx context.Context, tenant *model.Tenant, name string, labels environment.Labels) {
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
	if err := environment.SetLabels(ctx, env.ID, lbls); err != nil {
		d.t.Fatalf("set environment labels: %v", err)
	}
	if err := naisdstatus.Set(ctx, env.ID, &message.Health{ReportedAt: time.Now()}); err != nil {
		d.t.Fatalf("create health status: %v", err)
	}
}

func (d *reconcileDB) createTenantsAndEnvironments(ctx context.Context, tenantsAndEnvs map[string]environment.Labels) {
	d.t.Helper()
	tenants := make(map[string]*model.Tenant)
	for te, lbls := range tenantsAndEnvs {
		p := strings.Split(te, ":")
		tenantName, envName := p[0], p[1]
		if _, exists := tenants[tenantName]; !exists {
			tenant, err := environment.CreateTenant(ctx, &model.TenantCreate{Name: tenantName})
			if err != nil {
				d.t.Fatalf("create tenant: %v", err)
			}
			tenants[tenantName] = tenant
		}
		d.createEnv(ctx, tenants[tenantName], envName, lbls)
	}
}

func (d *reconcileDB) queryDeployedInstructions(ctx context.Context, t *testing.T) []string {
	t.Helper()
	rows, err := d.pool.Query(ctx, `
		SELECT t.name || ':' || e.name || ':' || di.feature_name || ':' || di.feature_version
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed'
		ORDER BY 1
	`)
	if err != nil {
		t.Fatalf("query deployed instructions: %v", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		result = append(result, s)
	}
	return result
}

func (d *reconcileDB) queryDeployedFeatures(ctx context.Context, t *testing.T, featureName string) []string {
	t.Helper()
	rows, err := d.pool.Query(ctx, `
		SELECT t.name || ':' || e.name
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed' AND di.feature_name = $1
		ORDER BY 1
	`, featureName)
	if err != nil {
		t.Fatalf("query deployed features: %v", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatalf("scan: %v", err)
		}
		result = append(result, s)
	}
	return result
}

func (d *reconcileDB) countInstructions(ctx context.Context, t *testing.T, featureName, version string) int {
	t.Helper()
	var count int
	err := d.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM deploy_instructions
		WHERE feature_name = $1 AND feature_version = $2
	`, featureName, version).Scan(&count)
	if err != nil {
		t.Fatalf("count instructions: %v", err)
	}
	return count
}

// startPostgresWithSnapshot starts a postgres container and takes a snapshot
// for restore between test cases.
func startPostgresWithSnapshot(ctx context.Context, t *testing.T) (container *postgres.PostgresContainer, dsn string) {
	t.Helper()
	container, dsn = startPostgres(ctx, t)
	if err := container.Snapshot(ctx); err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	return container, dsn
}

// setupReconcileTest creates a fresh DB, wires the reconciler, and returns
// everything needed to run a test case.
func setupReconcileTest(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string) (
	newCtx context.Context,
	db *reconcileDB,
	pub *sqlPublisher,
	reconcileFn reconcileFunc,
	seeder *featureassignmenttest.Seeder,
	lastResult **reconciler.DesiredState,
) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	pool, _, err := database.NewConnPool(ctx, dsn, logger)
	if err != nil {
		t.Fatalf("connect to database: %v", err)
	}
	t.Cleanup(func() {
		pool.Close()
		if err := container.Restore(ctx); err != nil {
			t.Fatalf("failed to restore database: %v", err)
		}
	})

	pub = &sqlPublisher{pool: pool}
	seeder = featureassignmenttest.NewSeeder()

	// Wire context loader for seeding.
	loadContext, err := contextloader.NewLoaderFunc(pool, logger)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}
	newCtx = loadContext(ctx)

	// Wire reconciler.
	newPub := func(topicID string, log *slog.Logger) reconciler.Publisher { return pub }
	dispatcher, err := reconciler.NewPubSubDispatcher(pool, newPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}
	rec, err := reconciler.New(pool, meter, logger)
	if err != nil {
		t.Fatalf("failed to create reconciler: %v", err)
	}
	var lr *reconciler.DesiredState
	lastResult = &lr
	reconcileFn = func(ctx context.Context) error {
		result, err := rec.ComputeDesiredState(ctx)
		if err != nil {
			return err
		}
		lr = result
		return dispatcher.Dispatch(ctx, result.Decisions)
	}

	db = &reconcileDB{t: t, pool: pool}
	return
}

type featureInput struct {
	name, version string
	dependencies  []string
	target        environment.Labels
}

// timedReconcileTest runs fn with the reconciler. Each reconcile call is timed and the duration is logged.
func timedReconcileTest(
	t *testing.T,
	ctx context.Context,
	container *postgres.PostgresContainer,
	dsn string,
	fn func(t *testing.T, ctx context.Context, db *reconcileDB, pub *sqlPublisher, reconcile reconcileFunc, seeder *featureassignmenttest.Seeder),
) {
	t.Helper()
	ctx, db, pub, reconcileFn, seeder, lastResultPtr := setupReconcileTest(ctx, t, container, dsn)
	featureassignment.ChartDownloader = seeder.ChartDownloader()

	timed := func(ctx context.Context) error {
		start := time.Now()
		err := reconcileFn(ctx)
		elapsed := time.Since(start)
		t.Logf("reconcile took %s", elapsed)
		if *lastResultPtr != nil {
			lr := *lastResultPtr
			ioDur := elapsed - lr.FetchDur - lr.ComputeDur
			t.Logf("  phases: fetch=%s compute=%s io=%s", lr.FetchDur, lr.ComputeDur, ioDur)
		}
		return err
	}

	fn(t, ctx, db, pub, timed, seeder)
}
