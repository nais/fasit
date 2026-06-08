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

	"github.com/google/uuid"
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

// sqlPublisher records published messages. naisd terminal-status reporting is
// simulated separately (see reconcileTest.reconcile) since the deploy_log row
// only exists after the deployer publishes.
type sqlPublisher struct {
	msg []message.DeployInstruction
}

func (p *sqlPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)
	return nil
}

func (p *sqlPublisher) Stop() {}

// tenantEnv describes one environment to create under a tenant.
type tenantEnv struct {
	tenant string
	name   string
	labels environment.Labels
}

// reconcileTest is a self-contained harness for a single reconciler test:
// a fresh database, a wired reconciler + deployer, a recording publisher,
// and a seeder. All methods fail the test directly on error.
type reconcileTest struct {
	t          *testing.T
	ctx        context.Context
	pool       *pgxpool.Pool
	pub        *sqlPublisher
	seeder     *featureassignmenttest.Seeder
	reconciler *reconciler.Reconciler
	deployer   reconciler.Deployer
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
// the reconciler, deployer, publisher and seeder, and registers cleanup that
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

	pub := &sqlPublisher{}
	newPub := func(topicID string, log *slog.Logger) reconciler.Publisher { return pub }
	deployer, err := reconciler.NewPubSubDeployer(pool, newPub, meter, logger)
	if err != nil {
		t.Fatalf("failed to create deployer: %v", err)
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
		deployer:   deployer,
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

// reconcile computes the desired state, deploys the resulting decisions, and
// simulates naisd reporting a terminal status for each newly published
// instruction (deployed, unless the feature name ends in "-pending", which
// stays in-progress).
func (h *reconcileTest) reconcile() {
	h.t.Helper()
	before := len(h.pub.msg)
	result, err := h.reconciler.ComputeDesiredState(h.ctx)
	if err != nil {
		h.t.Fatalf("reconcile: %v", err)
	}
	if err := h.deployer.Deploy(h.ctx, result.Results); err != nil {
		h.t.Fatalf("deploy: %v", err)
	}
	for _, msg := range h.pub.msg[before:] {
		if strings.HasSuffix(msg.Name, "-pending") {
			continue
		}
		h.appendDeployStatus(msg.ID, model.RolloutStatusDeployed)
	}
}

// appendDeployStatus mimics the naisd Receiver: it appends a terminal deploy_log
// row for the given diid, carrying the hash forward from the latest row.
func (h *reconcileTest) appendDeployStatus(diid uuid.UUID, status model.RolloutStatus) {
	h.t.Helper()
	_, err := h.pool.Exec(h.ctx, `
		INSERT INTO deploy_log (diid, environment_id, feature_assignment_id, feature_name, feature_version, status, hash)
		SELECT diid, environment_id, feature_assignment_id, feature_name, feature_version, $2, hash
		FROM deploy_log WHERE diid = $1 ORDER BY created DESC LIMIT 1
	`, diid, status.String())
	if err != nil {
		h.t.Fatalf("append deploy status: %v", err)
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

// deployedInstructions returns "tenant:env:feature:version" for every currently
// deployed feature×environment, sorted.
func (h *reconcileTest) deployedInstructions() []string {
	h.t.Helper()
	return h.queryStrings(`
		SELECT t.name || ':' || e.name || ':' || ds.feature_name || ':' || ds.feature_version
		FROM deploy_status ds
		JOIN environments e ON e.id = ds.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE ds.status = 'deployed'
		ORDER BY 1
	`)
}

// deployedFeatures returns "tenant:env" for every currently deployed
// environment of the given feature, sorted.
func (h *reconcileTest) deployedFeatures(featureName string) []string {
	h.t.Helper()
	return h.queryStrings(`
		SELECT t.name || ':' || e.name
		FROM deploy_status ds
		JOIN environments e ON e.id = ds.environment_id
		JOIN tenants t ON t.id = e.tenant_id
		WHERE ds.status = 'deployed' AND ds.feature_name = $1
		ORDER BY 1
	`, featureName)
}

func (h *reconcileTest) countInstructions(featureName, version string) int {
	h.t.Helper()
	var count int
	err := h.pool.QueryRow(h.ctx, `
		SELECT COUNT(DISTINCT diid) FROM deploy_log
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
