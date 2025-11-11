package deployment_test

import (
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/dbtest"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

// Intentional uppercase to avoid var clashes
type Db struct {
	repo database.Repo
	t    *testing.T
}

type featureDeploy struct {
	name    string
	version string
	target  environment.Labels
}

type featureInput struct {
	name, version string
	dependencies  []string
	labels        map[string]string
}

func TestMultipleReconcile(t *testing.T) {
	ctx := context.Background()
	db := setupDb(ctx, t, true)

	r, err := deployment.NewReconciler(
		db.repo,
		func(topicID string, log *logrus.Entry) deployment.Publisher {
			return nil
		},
		nil,
		nil,
		logrus.NewEntry(logrus.New()),
	)
	if err != nil {
		t.Fatalf("create reconciler: %v", err)
	}

	features := []featureInput{
		{
			name:    "monitoring",
			version: "v1",
			dependencies: []string{
				"monitoring-crds",
			},
		},
		{
			name:    "monitoring-crds",
			version: "v1",
			// TODO: how to deploy a feature everywhere
		},
	}

	envsToCreate := map[string]map[string]environment.Labels{
		"nav": {
			"dev": environment.Labels{
				"aiven": "enabled",
				"kind":  "tenant",
			},
			"management": environment.Labels{
				"kind": "management",
			},
		},
	}

	db.createTenantEnvironments(ctx, envsToCreate)
	db.createFeatureDeployments(ctx, features)

	targetsAfterReconcile := [][]string{
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
	}

	for _, targets := range targetsAfterReconcile {
		err = r.Reconcile(ctx)
		if err != nil {
			t.Fatalf("reconcile: %v", err)
		}

		allDeploymentTargets, err := db.repo.DeploymentTargetsGetAll(ctx)
		if err != nil {
			t.Fatalf("get all deployment targets: %v", err)
		}
		assert.Len(t, allDeploymentTargets, len(targets))
		for _, dt := range allDeploymentTargets {
			assert.Contains(t, targets, fmt.Sprintf("%s:%s:%s:%s", dt.TenantName, dt.EnvironmentName, dt.FeatureName, dt.Version))
		}
	}
	fmt.Printf("%d features created\n", len(features))

}

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	db := setupDb(ctx, t, true)

	r, err := deployment.NewReconciler(
		db.repo,
		func(topicID string, log *logrus.Entry) deployment.Publisher {
			return nil
		},
		nil,
		nil,
		logrus.NewEntry(logrus.New()),
	)
	if err != nil {
		t.Fatalf("create reconciler: %v", err)
	}

	type tenantKey = string
	envsToCreate := map[tenantKey]map[string]environment.Labels{
		"test-partner": {
			"dev": environment.Labels{},
			"prod": environment.Labels{
				"featuretoggle": "enabled",
			},
		},
		"nav": {
			"dev": environment.Labels{
				"aiven": "enabled",
			},
			"management": environment.Labels{
				"kind": "management",
			},
		},
	}

	tt := []struct {
		name                      string
		deploymentsToCreate       []featureDeploy
		expectedDeploymentTargets []map[string]map[string][]featureDeploy
	}{
		{
			name: "install most specific and latest features",
			deploymentsToCreate: []featureDeploy{
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
			expectedDeploymentTargets: []map[string]map[string][]featureDeploy{
				{
					"nav": {
						"dev": {
							{name: "aivenator", version: "1.1.1", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
							{name: "naiserator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
						"management": {
							{name: "v13s", version: "1.0.0", target: environment.Labels{"kind": "management"}},
						},
					},
					"test-partner": {
						"dev": {
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
						"prod": {
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
					},
				},
				{
					"nav": {
						"dev": {
							{name: "aivenator", version: "1.1.1", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
							{name: "naiserator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
						"management": {
							{name: "v13s", version: "1.0.0", target: environment.Labels{"kind": "management"}},
						},
					},
					"test-partner": {
						"dev": {
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
						"prod": {
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
					},
				},
				{
					"nav": {
						"dev": {
							{name: "aivenator", version: "1.1.1", target: environment.Labels{"aiven": "enabled", "tenant": "nav"}},
							{name: "naiserator", version: "1.0.0", target: environment.Labels{"aiven": "enabled"}},
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
						"management": {
							{name: "v13s", version: "1.0.0", target: environment.Labels{"kind": "management"}},
						},
					},
					"test-partner": {
						"dev": {
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
						"prod": {
							{name: "unleash", version: "2.0.0", target: environment.Labels{"kind": "tenant"}},
						},
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			db.createTenantEnvironments(ctx, envsToCreate)
			db.createDeployments(ctx, tc.deploymentsToCreate)

			for _, expectedTargets := range tc.expectedDeploymentTargets {
				err = r.Reconcile(ctx)
				if err != nil {
					t.Fatalf("reconcile: %v", err)
				}

				allDeploymentTargets, err := db.repo.DeploymentTargetsGetAll(ctx)
				if err != nil {
					t.Fatalf("get all deployment targets: %v", err)
				}

				countExpected := 0
				for _, envs := range expectedTargets {
					for _, dts := range envs {
						countExpected += len(dts)
					}
				}
				assert.Len(t, allDeploymentTargets, countExpected, "expected %d deployment targets, got %d", countExpected, len(allDeploymentTargets))

				count := 0
				for _, dt := range allDeploymentTargets {
					featureDeploys, ok := expectedTargets[dt.TenantName][dt.EnvironmentName]
					if !ok {
						t.Fatalf(
							"%s:%s not found in expected deployment targets map",
							dt.TenantName,
							dt.EnvironmentName,
						)
					}
					found := false
					for _, fd := range featureDeploys {
						if dt.FeatureName == fd.name && dt.Version == fd.version {
							count++
							found = true
							break
						}
					}
					assert.True(t, found,
						"deployment target %s:%s:%s:%s not found in expected deployment targets",
						dt.TenantName,
						dt.EnvironmentName,
						dt.FeatureName,
						dt.Version,
					)
				}
				assert.Len(t, allDeploymentTargets, count, "number of deployment targets does not match the expected number of deployment targets")
			}
		})
	}
}

func (d *Db) createDeployment(ctx context.Context, featureName, version string, labels environment.Labels) {
	d.t.Helper()
	err := d.repo.FeatureDataCreate(ctx, model.Feature{
		FeatureYAML: model.FeatureYAML{},
		Name:        featureName,
		Version:     version,
		Chart:       "oci://" + featureName,
	}, &feature.FeatureTemplateDetails{})
	if err != nil && !strings.Contains(err.Error(), "SQLSTATE 23505") {
		d.t.Fatalf("create feature data: %v", err)
	}

	err = d.repo.FeatureVersionUpdate(ctx, featureName, version)
	if err != nil {
		d.t.Fatalf("update feature version: %v", err)
	}

	_, err = d.repo.DeploymentCreate(ctx, featureName, version, []byte(`{"key": "ghref"}`), labels, "TODO: hash")
	if err != nil {
		d.t.Fatalf("create deployment: %v", err)
	}
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
		d.createFeatureDeployment(ctx, f.name, f.version, f.labels, deps)
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

	_, err = d.repo.DeploymentCreate(ctx, featureName, version, []byte(`{"key": "ghref"}`), labels, "TODO: hash")
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
func (d *Db) createTenantEnvironments(ctx context.Context, envsToCreate map[string]map[string]environment.Labels) {
	d.t.Helper()
	for k, v := range envsToCreate {
		tenant, err := d.repo.TenantCreate(ctx, &model.TenantCreate{
			Name: k,
		})
		if err != nil {
			d.t.Fatalf("create tenant: %v", err)
		}
		for name, labels := range v {
			d.createEnv(ctx, tenant, name, labels)
		}
	}
}

func (d *Db) createDeployments(ctx context.Context, deployments []featureDeploy) {
	for _, deploy := range deployments {
		d.createDeployment(ctx, deploy.name, deploy.version, deploy.target)
	}
}

func setupDb(ctx context.Context, t *testing.T, testcontainers bool) *Db {
	t.Helper()

	log := logrus.New()
	log.Out = io.Discard

	var pool *pgxpool.Pool
	dbs := "postgres://postgres:postgres@127.0.0.1:5432/fasit?sslmode=disable"
	if !testcontainers {
		p, c, err := database.NewDB(ctx, dbs, false)
		if err != nil {
			t.Fatalf("Error connecting to database: %v", err)
		}
		pool = p
		t.Cleanup(func() {
			_ = c.Close()
		})
	} else {
		d, c := dbtest.DockerSQLPool(ctx)
		t.Logf("PostgreSQL DSN: %q", d)
		dbs = d
		db, closers, err := database.NewDB(ctx, dbs, false)
		if err != nil {
			log.Fatalf("Error connecting to database: %v", err)
		}
		t.Cleanup(func() {
			_ = closers.Close()
			c()
		})
		pool = db
	}

	if err := database.Migrate("pgx", dbs, logrus.NewEntry(log)); err != nil {
		t.Fatalf("Could not migrate: %v", err)
	}
	return &Db{
		repo: database.New(pool, log.WithField("subsystem", "repo")),
		t:    t,
	}
}
