package deployment_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Intentional uppercase to avoid var clashes
type Db struct {
	repo database.Repo
	t    *testing.T
	pool *pgxpool.Pool
}

type featureInput struct {
	name, version string
	dependencies  []string
	target        environment.Labels
}

func TestReconcile(t *testing.T) {
	ctx := context.Background()

	container, dsn, err := startPostgresql(ctx, t)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	logger, _ := test.NewNullLogger()

	envsToCreate := map[string]environment.Labels{
		"test-partner:dev":  {},
		"test-partner:prod": {"featuretoggle": "enabled"},
		"nav:dev":           {"aiven": "enabled"},
		"nav:management":    {"kind": "management"},
	}

	tt := []struct {
		name                string
		deploymentsToCreate []featureInput
		reconcileResults    [][]string
	}{
		{
			name: "install most specific and latest features",
			deploymentsToCreate: []featureInput{
				{name: "aivenator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "aivenator", version: "2.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "aivenator", version: "1.1.0", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
				{name: "aivenator", version: "1.1.1", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
				{name: "aivenator", version: "3.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "naiserator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
				{name: "unleash", version: "1.0.0", target: environment.Labels{"featuretoggle": "enabled"}},
				{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
				{name: "v13s", version: "1.0.0", target: environment.Labels{"kind": "management"}},
			},
			reconcileResults: [][]string{
				{
					"nav:dev:aivenator:1.1.1",
					"nav:dev:naiserator:1.0.0",
					"nav:dev:unleash:2.0.0",
					"nav:management:v13s:1.0.0",
					"test-partner:dev:unleash:2.0.0",
					"test-partner:prod:unleash:2.0.0",
				},
			},
		},
		{
			name: "install features with dependencies",
			deploymentsToCreate: []featureInput{
				{
					name:    "monitoring",
					version: "v1",
					dependencies: []string{
						"monitoring-crds",
					},
					target: environment.Labels{"tenant": "nav"},
				},
				{
					name:    "monitoring-crds",
					version: "v1",
					target:  environment.Labels{"tenant": "nav"},
				},
			},
			reconcileResults: [][]string{
				{
					"nav:dev:monitoring-crds:v1",
					"nav:management:monitoring-crds:v1",
				},
				{
					"nav:dev:monitoring-crds:v1",
					"nav:management:monitoring-crds:v1",
					"nav:dev:monitoring:v1",
					"nav:managment:monitoring:v1",
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			fmt.Printf("dsn: %s\n", dsn)
			db := getConnection(ctx, t, container, dsn, logger)
			pub := &publisher{db: db}
			r, err := deployment.NewReconciler(
				db.repo,
				func(topicID string, log logrus.FieldLogger) deployment.Publisher {
					return pub
				},
				nil,
				nil,
				logger,
			)
			if err != nil {
				t.Fatalf("create reconciler: %v", err)
			}

			db.createTenantEnvironments(ctx, envsToCreate)
			db.createFeatureDeployments(ctx, tc.deploymentsToCreate)

			for _, result := range tc.reconcileResults {
				if err = r.Reconcile(ctx); err != nil {
					t.Fatalf("reconcile: %v", err)
				}
				q := `
				SELECT
				t.name || ':' || e.name || ':' || di.feature_name || ':' || di.feature_version
				FROM deploy_instructions di
				JOIN environments e ON e.id = di.environment_id
				JOIN tenants t ON t.id = e.tenant_id
			`

				rows, err := db.runQuery(ctx, q)
				if err != nil {
					t.Fatalf("query deploy instructions: %v", err)
				}

				var instructions []string
				for rows.Next() {
					var instruction string
					_ = rows.Scan(&instruction)
					instructions = append(instructions, instruction)
				}

				assert.Len(t, instructions, len(result))
				assert.Len(t, pub.msg, len(result))

				for _, instruction := range result {
					found := false
					for _, msg := range pub.msg {
						parts := strings.Split(instruction, ":")
						if fmt.Sprintf("%s:%s", msg.Name, msg.Version) == strings.Join(parts[2:], ":") {
							found = true
							break
						}
					}

					if !found {
						t.Errorf("expected log message for instruction %q not found", instruction)
					}
				}
			}
		})
	}
}

type publisher struct {
	msg []message.DeployInstruction
	db  Db
}

func (p *publisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	p.msg = append(p.msg, msg)
	return p.db.repo.DeployInstructionUpdateStatus(ctx, msg.ID, model.RolloutStatusDeployed)
}

func (p *publisher) Stop() {}

func (d Db) runQuery(ctx context.Context, q string) (pgx.Rows, error) {
	return d.pool.Query(ctx, q)
}

func (d *Db) createFeatureDeployments(ctx context.Context, features []featureInput) {
	for _, f := range features {
		var deps model.Dependencies
		if len(f.dependencies) > 0 {
			deps = model.Dependencies{
				&model.Dependency{
					AllOf: f.dependencies,
				},
			}
		}
		d.createFeatureDeployment(ctx, f.name, f.version, f.target, deps)
	}
}

func (d *Db) createFeatureDeployment(ctx context.Context, featureName, version string, labels environment.Labels, dependencies model.Dependencies) {
	d.t.Helper()
	err := d.repo.FeatureDataCreate(ctx, model.Feature{
		FeatureYAML: model.FeatureYAML{
			Dependencies: dependencies,
		},
		Name:    featureName,
		Version: version,
		Chart:   "oci://" + featureName,
	}, &feature.FeatureTemplateDetails{})
	if err != nil && !strings.Contains(err.Error(), "SQLSTATE 23505") {
		d.t.Fatalf("create feature data: %v", err)
	}

	err = d.repo.FeatureVersionUpdate(ctx, featureName, version)
	if err != nil {
		d.t.Fatalf("update feature version: %v", err)
	}

	if labels == nil {
		labels = environment.Labels{}
	}

	_, err = d.repo.DeploymentCreate(ctx, featureName, version, []byte(`{"key": "ghref"}`), labels)
	if err != nil {
		d.t.Fatalf("create deployment: %v", err)
	}
}

func (d *Db) createEnv(ctx context.Context, tenant *model.Tenant, name string, labels environment.Labels) {
	d.t.Helper()

	if labels["kind"] == "" {
		labels["kind"] = "tenant"
	}
	env, err := d.repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     name,
		TenantID: tenant.ID,
		Kind:     model.EnvironmentKind(labels["kind"]),
	})
	if err != nil {
		d.t.Fatalf("create environment: %v", err)
	}
	lbls := make([]*protogen.EnvironmentLabel, 0)
	for k, v := range labels {
		lbls = append(lbls, &protogen.EnvironmentLabel{
			Key:   k,
			Value: v,
		})
	}
	lbls = append(lbls, &protogen.EnvironmentLabel{
		Key:   "tenant",
		Value: tenant.Name,
	}, &protogen.EnvironmentLabel{
		Key:   "environment",
		Value: name,
	})
	err = d.repo.EnvironmentSetLabels(ctx, env.ID, lbls)
	if err != nil {
		d.t.Fatalf("set environment labels: %v", err)
	}

	err = d.repo.HealthStatusCreateOrUpdate(ctx, env.ID, &message.Health{
		ReportedAt: time.Now(),
	})
}

// key in envsToCreate is tenantName, key in inner map is env name
func (d *Db) createTenantEnvironments(ctx context.Context, envsToCreate map[string]environment.Labels) {
	d.t.Helper()

	for k, v := range envsToCreate {
		tenantName := strings.Split(k, ":")[0]
		envName := strings.Split(k, ":")[1]
		tenant, _ := d.repo.TenantCreate(ctx, &model.TenantCreate{
			Name: tenantName,
		})
		d.createEnv(ctx, tenant, envName, v)
	}
}

func startPostgresql(ctx context.Context, t *testing.T) (container *postgres.PostgresContainer, dsn string, err error) {
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
	if err = database.Migrate("pgx", dsn, logger); err != nil {
		return nil, "", fmt.Errorf("failed to migrate database: %w", err)
	}

	if err = container.Snapshot(ctx); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, dsn, nil
}

func getConnection(ctx context.Context, t *testing.T, container *postgres.PostgresContainer, dsn string, log logrus.FieldLogger) Db {
	pool, _, err := database.NewDB(ctx, dsn, false)
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
		repo: database.New(pool, log),
		t:    t,
		pool: pool,
	}
}
