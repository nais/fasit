package deployment_test

import (
	"context"
	"io"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/dbtest"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/provider/protogen"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestReconcile(t *testing.T) {
	ctx := context.Background()
	db := setupDb(ctx, t, true)

	r, err := deployment.NewReconciler(
		db,
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

	const tenantName = "tenant-1"

	tenant, err := db.TenantCreate(ctx, &model.TenantCreate{
		Name: tenantName,
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	envsWithAiven := make([]uuid.UUID, 0)
	envsWithUnleash := make([]uuid.UUID, 0)

	{
		env, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
			Name:     "management",
			TenantID: tenant.ID,
			Kind:     "management",
		})
		if err != nil {
			t.Fatalf("create environment: %v", err)
		}
		err = db.EnvironmentSetLabels(ctx, env.ID, []*protogen.EnvironmentLabel{
			{
				Key:   "tenant",
				Value: tenantName,
			},
			{
				Key:   "environment",
				Value: "management",
			},
		})
		if err != nil {
			t.Fatalf("set environment labels: %v", err)
		}
	}

	{
		env, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
			Name:     "dev",
			TenantID: tenant.ID,
			Kind:     "tenant",
		})
		if err != nil {
			t.Fatalf("create environment: %v", err)
		}
		err = db.EnvironmentSetLabels(ctx, env.ID, []*protogen.EnvironmentLabel{
			{
				Key:   "tenant",
				Value: tenantName,
			},
			{
				Key:   "environment",
				Value: "dev",
			},
			{
				Key:   "aiven",
				Value: "true",
			},
		})
		if err != nil {
			t.Fatalf("set environment labels: %v", err)
		}

		envsWithAiven = append(envsWithAiven, env.ID)
	}

	{
		env, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
			Name:     "prod",
			TenantID: tenant.ID,
			Kind:     "tenant",
		})
		if err != nil {
			t.Fatalf("create environment: %v", err)
		}
		err = db.EnvironmentSetLabels(ctx, env.ID, []*protogen.EnvironmentLabel{
			{
				Key:   "tenant",
				Value: tenantName,
			},
			{
				Key:   "environment",
				Value: "prod",
			},
			{
				Key:   "aiven",
				Value: "true",
			},
			{
				Key:   "unleash",
				Value: "true",
			},
		})
		if err != nil {
			t.Fatalf("set environment labels: %v", err)
		}

		envsWithAiven = append(envsWithAiven, env.ID)
		envsWithUnleash = append(envsWithUnleash, env.ID)
	}

	tenant, err = db.TenantCreate(ctx, &model.TenantCreate{
		Name: "tenant-2",
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	{
		env, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
			Name:     "prod",
			TenantID: tenant.ID,
			Kind:     "tenant",
		})
		if err != nil {
			t.Fatalf("create environment: %v", err)
		}
		err = db.EnvironmentSetLabels(ctx, env.ID, []*protogen.EnvironmentLabel{
			{
				Key:   "tenant",
				Value: "tenant-2",
			},
			{
				Key:   "environment",
				Value: "prod",
			},
			{
				Key:   "aiven",
				Value: "true",
			},
			{
				Key:   "unleash",
				Value: "true",
			},
		})
		if err != nil {
			t.Fatalf("set environment labels: %v", err)
		}

		envsWithAiven = append(envsWithAiven, env.ID)
	}

	err = db.FeatureDataCreate(ctx, model.Feature{
		Name:    "aiven",
		Version: "v2",
		Chart:   "oci://aiven",
	}, &feature.FeatureTemplateDetails{})
	if err != nil {
		t.Fatalf("create feature data: %v", err)
	}

	err = db.FeatureDataCreate(ctx, model.Feature{
		Name:    "unleash",
		Version: "v3",
		Chart:   "oci://unleash",
	}, &feature.FeatureTemplateDetails{})
	if err != nil {
		t.Fatalf("create feature data: %v", err)
	}

	dep1, err := db.DeploymentCreate(ctx, "aiven", "v2", []byte(`{"key": "ghref"}`), database.EnvironmentLabels{
		"aiven": "true",
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	dep2, err := db.DeploymentCreate(ctx, "unleash", "v3", []byte(`{"key": "ghref"}`), database.EnvironmentLabels{
		"tenant":  tenantName,
		"unleash": "true",
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}

	deploys, err := db.DeploymentsGet(ctx)
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}
	assert.Len(t, deploys, 2)

	if err := r.Reconcile(ctx); err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	targets, err := db.DeploymentTargetsGet(ctx, dep1.ID)
	if err != nil {
		t.Fatalf("get deployment targets: %v", err)
	}
	assert.Len(t, targets, 3)

	for _, target := range targets {
		assert.Contains(t, envsWithAiven, target.EnvironmentID, "environment ID should be in envsWithAiven")
	}

	targets, err = db.DeploymentTargetsGet(ctx, dep2.ID)
	if err != nil {
		t.Fatalf("get deployment targets: %v", err)
	}
	assert.Len(t, targets, 1)

	for _, target := range targets {
		assert.Contains(t, envsWithUnleash, target.EnvironmentID, "environment ID should be in envsWithUnleash")
	}
}

func setupDb(ctx context.Context, t *testing.T, testcontainers bool) database.Repo {
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
	return database.New(pool, log.WithField("subsystem", "repo"))
}
