//go:build integration_test_old

package workers

import (
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

func TestRolloutIntegration_run(t *testing.T) {
	// This test uses a docker pool for the database to be able to run with transactions.
	db, dbString, cleanup := dbtest.DockerSQLPool()
	defer cleanup()

	logger := logrus.New()
	logger.Out = io.Discard
	log := logrus.NewEntry(logger)
	if err := database.Migrate(db, log); err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}
	db, err := database.NewDB("postgres", dbString)
	if err != nil {
		t.Fatalf("Could not create db: %v", err)
	}

	ctx := context.Background()
	tenantEnvID := uuid.New()
	managementEnvID := uuid.New()
	tenantID := uuid.New()

	db.ExecContext(ctx, `INSERT INTO tenants (id, name, ci) VALUES ($1, 'tenant1', true)`, tenantID)
	db.ExecContext(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, 'env1', 'tenant', true)`, tenantEnvID, tenantID)
	db.ExecContext(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, 'env1', 'management', true)`, managementEnvID, tenantID)

	repo := database.New(db, uuid.NewString(), log.WithField("test", "rollout"))

	worker := NewRollout(repo, log)

	t.Run("nothing done when no new rollout", func(t *testing.T) {
		if err := worker.run(ctx); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
	})

	t.Run("rollout is created", func(t *testing.T) {
		if err := repo.EnvironmentValueStore(ctx, tenantEnvID, "tag", []byte(`"old"`), false); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		roll, err := repo.RolloutCreate(ctx, &model.Rollout{
			Feature: "rollout_create",
			Changeset: &model.RolloutChangeset{
				New: map[string]json.RawMessage{"tag": []byte(`"v1"`)},
			},
		})
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		if err := worker.run(ctx); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		roll, err = repo.RolloutGetByID(ctx, roll.ID)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}

		want := &model.Rollout{
			ID:      roll.ID,
			Feature: "rollout_create",
			Status:  model.RolloutStatusPending,
			Changeset: &model.RolloutChangeset{
				New: map[string]json.RawMessage{"tag": []byte(`"v1"`)},
				Old: map[string]json.RawMessage{"tag": []byte(`"old"`)},
			},
		}
		if !cmp.Equal(want, roll, cmpopts.IgnoreFields(model.Rollout{}, "Created", "LastModified")) {
			t.Errorf("diff -want +got:\n%v", cmp.Diff(want, roll))
		}
	})
}

// func TestRolloutIntegration(t *testing.T) {
// 	ctx := context.Background()
// 	ctx, cancel := context.WithCancel(ctx)
// 	defer cancel()

// 	envID := uuid.New()
// 	tenantID := uuid.New()
// 	q1 := `INSERT INTO tenants (id, name, ci) VALUES ('%s', 'tenant1', true)`
// 	q2 := `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ('%s', '%s', 'env1', 'tenant', true)`

// 	repo := newTestRepo(t,
// 		fmt.Sprintf(q1, tenantID),
// 		fmt.Sprintf(q2, envID, tenantID),
// 	)

// 	rollout := NewRollout(repo, logrus.WithField("test", "rollout"))

// 	// The following tests does not use the postgres trigger
// 	t.Run("no rollouts", func(t *testing.T) {
// 		if err := rollout.run(ctx); err != nil {
// 			t.Fatal(err)
// 		}
// 	})

// 	t.Run("new rollout without existing config", func(t *testing.T) {
// 		nr, err := repo.RolloutCreate(ctx, &model.Rollout{
// 			Feature: "testfeature",
// 			Changeset: &model.RolloutChangeset{
// 				New: map[string]json.RawMessage{
// 					"image": json.RawMessage(`{"tag": "latest"}`),
// 				},
// 			},
// 		})
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		if err := rollout.run(ctx); err != nil {
// 			t.Fatal(err)
// 		}

// 		rollout, err := repo.RolloutGetByID(ctx, nr.ID)
// 		if err != nil {
// 			t.Fatal(err)
// 		}

// 		if rollout.Status != model.RolloutStatusPending {
// 			t.Fatalf("expected rollout to be completed, got %v", rollout.Status)
// 		}
// 	})

// 	// The following tests uses the postgres trigger

// 	go rollout.Listen(ctx)
// }

// func TestMain(m *testing.M) {
// 	db, dbString, cleanup := dbtest.DockerSQLPool()

// 	log := logrus.New()
// 	log.Out = io.Discard
// 	if err := database.Migrate(db, logrus.NewEntry(log)); err != nil {
// 		log.Fatalf("Could not migrate: %v", err)
// 	}
// 	txdb.Register("txdb_rollout", "postgres", dbString)

// 	code := m.Run()

// 	cleanup()

// 	os.Exit(code)
// }

// func newTestRepo(t testing.TB, stmts ...string) database.Repo {
// 	t.Helper()

// 	db, err := database.NewDB("txdb", uuid.NewString())
// 	if err != nil {
// 		t.Fatalf("Could not create db: %v", err)
// 	}

// 	r := database.New(db, uuid.NewString(), logrus.WithField("test", "rollout"))
// 	for _, s := range stmts {
// 		_, err := db.Exec(s)
// 		if err != nil {
// 			t.Fatalf("Error executing:\n%v\nErr: %v", s, err)
// 		}
// 	}
// 	return r
// }
