//go:build integration_test

package reconciler_test

import (
	"context"
	"io"
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/featureassignment/featureassignmenttest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler"

	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

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

// tenantEnv describes one environment to create under a tenant.
type tenantEnv struct {
	tenant string
	name   string
	labels environment.Labels
}

// reconcileTest is a self-contained harness for a single reconciler test:
// a fresh database, a wired reconciler + dispatcher, a recording publisher,
// and a seeder. All methods fail the test directly on error.
type reconcileTest struct {
	t          *testing.T
	ctx        context.Context
	pool       *pgxpool.Pool
	pub        *sqlPublisher
	seeder     *featureassignmenttest.Seeder
	reconciler *reconciler.Reconciler
	dispatcher reconciler.Dispatcher
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

// newReconcileTest opens a fresh connection to the snapshotted database, wires
// the reconciler, dispatcher, publisher and seeder, and registers cleanup that
// restores the snapshot for the next case.
func newReconcileTest(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string) *reconcileTest {
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

	loadContext, err := contextloader.NewLoaderFunc(pool, logger)
	if err != nil {
		t.Fatalf("failed to create loader: %v", err)
	}

	pub := &sqlPublisher{pool: pool}
	newPub := func(topicID string, log *slog.Logger) reconciler.Publisher { return pub }
	dispatcher, err := reconciler.NewPubSubDispatcher(pool, newPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create dispatcher: %v", err)
	}
	rec, err := reconciler.New(pool, meter, logger)
	if err != nil {
		t.Fatalf("failed to create reconciler: %v", err)
	}

	seeder := featureassignmenttest.NewSeeder()
	featureassignment.ChartDownloader = seeder.ChartDownloader()

	return &reconcileTest{
		t:          t,
		ctx:        loadContext(ctx),
		pool:       pool,
		pub:        pub,
		seeder:     seeder,
		reconciler: rec,
		dispatcher: dispatcher,
	}
}

// createAssignment registers a fake chart and creates the assignment immediately.
func (h *reconcileTest) createAssignment(name, version string, target environment.Labels, deps ...string) {
	h.t.Helper()
	if _, err := h.seeder.CreateAssignment(h.ctx, name, version, target, deps...); err != nil {
		h.t.Fatalf("create assignment %s@%s: %v", name, version, err)
	}
}

// createAssignmentWithValues is createAssignment with configurable values and
// fake chart defaults.
func (h *reconcileTest) createAssignmentWithValues(name, version string, target environment.Labels, kinds []environment.EnvironmentKind, values feature.Values, defaults map[string]any, description string, deps ...string) {
	h.t.Helper()
	if _, err := h.seeder.CreateAssignmentWithValues(h.ctx, name, version, target, kinds, values, defaults, description, deps...); err != nil {
		h.t.Fatalf("create assignment %s@%s: %v", name, version, err)
	}
}

// reconcile computes the desired state and dispatches the resulting decisions.
func (h *reconcileTest) reconcile() {
	h.t.Helper()
	result, err := h.reconciler.ComputeDesiredState(h.ctx)
	if err != nil {
		h.t.Fatalf("reconcile: %v", err)
	}
	if err := h.dispatcher.Dispatch(h.ctx, result.Decisions); err != nil {
		h.t.Fatalf("dispatch: %v", err)
	}
}

func (h *reconcileTest) createEnvs(envs ...tenantEnv) {
	h.t.Helper()
	tenants := make(map[string]*environment.Tenant)
	for _, e := range envs {
		tenant, exists := tenants[e.tenant]
		if !exists {
			var err error
			tenant, err = environment.CreateTenant(h.ctx, &environment.TenantCreate{Name: e.tenant})
			if err != nil {
				h.t.Fatalf("create tenant: %v", err)
			}
			tenants[e.tenant] = tenant
		}
		h.createEnv(tenant, e.name, e.labels)
	}
}

func (h *reconcileTest) createEnv(tenant *environment.Tenant, name string, labels environment.Labels) {
	h.t.Helper()
	if labels["kind"] == "" {
		labels["kind"] = "tenant"
	}
	env, err := environment.Create(h.ctx, &environment.EnvironmentCreate{
		Name:     name,
		TenantID: tenant.ID,
		Kind:     environment.EnvironmentKind(labels["kind"]),
	})
	if err != nil {
		h.t.Fatalf("create environment: %v", err)
	}
	lbls := environment.Labels{}
	maps.Copy(lbls, labels)
	lbls["tenant"] = tenant.Name
	lbls["environment"] = env.Name
	if err := environment.SetLabels(h.ctx, env.ID, lbls); err != nil {
		h.t.Fatalf("set environment labels: %v", err)
	}
	if err := naisdstatus.Set(h.ctx, env.ID, &message.Health{ReportedAt: time.Now()}); err != nil {
		h.t.Fatalf("create health status: %v", err)
	}
}

// deployedInstructions returns "tenant:env:feature:version" for every deployed
// instruction, sorted.
func (h *reconcileTest) deployedInstructions() []string {
	h.t.Helper()
	return h.queryStrings(`
		SELECT t.name || ':' || e.name || ':' || di.feature_name || ':' || di.feature_version
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed'
		ORDER BY 1
	`)
}

// deployedFeatures returns "tenant:env" for every deployed instruction of the
// given feature, sorted.
func (h *reconcileTest) deployedFeatures(featureName string) []string {
	h.t.Helper()
	return h.queryStrings(`
		SELECT t.name || ':' || e.name
		FROM deploy_instructions di
		JOIN environments e ON e.id = di.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE di.status = 'deployed' AND di.feature_name = $1
		ORDER BY 1
	`, featureName)
}

func (h *reconcileTest) countInstructions(featureName, version string) int {
	h.t.Helper()
	var count int
	err := h.pool.QueryRow(h.ctx, `
		SELECT COUNT(*) FROM deploy_instructions
		WHERE feature_name = $1 AND feature_version = $2
	`, featureName, version).Scan(&count)
	if err != nil {
		h.t.Fatalf("count instructions: %v", err)
	}
	return count
}

func (h *reconcileTest) queryStrings(sql string, args ...any) []string {
	h.t.Helper()
	rows, err := h.pool.Query(h.ctx, sql, args...)
	if err != nil {
		h.t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			h.t.Fatalf("scan: %v", err)
		}
		result = append(result, s)
	}
	return result
}

// requireDeployed asserts that the set of deployed instructions
// ("tenant:env:feature:version") is exactly want.
func (h *reconcileTest) requireDeployed(want ...string) {
	h.t.Helper()
	sorted := slices.Clone(want)
	sort.Strings(sorted)
	got := h.deployedInstructions()
	if !slices.Equal(got, sorted) {
		h.t.Errorf("deployed instructions mismatch:\ngot:  %v\nwant: %v", got, sorted)
	}
}

// requirePublished asserts the number of messages published since the last reset.
func (h *reconcileTest) requirePublished(n int) {
	h.t.Helper()
	if len(h.pub.msg) != n {
		h.t.Fatalf("published messages = %d, want %d", len(h.pub.msg), n)
	}
}
