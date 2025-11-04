package deployment_test

import (
	"context"
	"encoding/json"
	"io"
	"testing"

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

	tenant, err := db.TenantCreate(ctx, &model.TenantCreate{
		Name: "tenant-1",
	})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}

	_, err = db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "env-no-labels",
		TenantID: tenant.ID,
		Kind:     "tenant",
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	env, err := db.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "env-with-labels",
		TenantID: tenant.ID,
		Kind:     "tenant",
	})
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}

	err = db.EnvironmentSetLabels(ctx, env.ID, []*protogen.EnvironmentLabel{
		{
			Key:   "foo",
			Value: "bar",
		},
	})
	if err != nil {
		t.Fatalf("set environment labels: %v", err)
	}

	err = db.FeatureDataCreate(ctx, model.Feature{
		FeatureYAML: model.FeatureYAML{},
		Name:        "feature-a",
		Version:     "v2",
		Chart:       "oci://feature-a-chart",
		Description: "Feature A description",
		Source:      "sr",
		ValuesYAML:  make(map[string]json.RawMessage),
		SpecVersion: "v2",
	}, &feature.FeatureTemplateDetails{})
	if err != nil {
		t.Fatalf("create feature data: %v", err)
	}

	err = db.DeploymentCreate(ctx, "feature-a", "v2", []byte(`{"key": "ghref"}`), database.EnvironmentLabels{
		"foo": "bar",
	})
	if err != nil {
		t.Fatalf("create deployment: %v", err)
	}
	deploys, err := db.DeploymentsGet(ctx)
	if err != nil {
		t.Fatalf("get deployment: %v", err)
	}

	err = r.Reconcile(ctx)
	if err != nil {
		t.Fatalf("Reconcile failed: %v", err)
	}

	targets, err := db.DeploymentTargetsGet(ctx)
	if err != nil {
		t.Fatalf("get deployment targets: %v", err)
	}

	assert.Len(t, targets, 1)
	assert.Equal(t, targets[0].EnvironmentID, env.ID)
	assert.Len(t, deploys, 1)
	assert.Equal(t, deploys[0].ID, targets[0].DeploymentID)
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
