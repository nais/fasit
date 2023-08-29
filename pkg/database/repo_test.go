//go:build integration_test

package database

import (
	"context"
	"io"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/sirupsen/logrus"
)

var (
	dbString   = ""
	repository *repo
)

func TestMain(m *testing.M) {
	dbs, cleanup := dbtest.DockerSQLPool()
	dbString = dbs

	log := logrus.New()
	log.Out = io.Discard
	ctx := context.Background()

	db, closers, err := NewDB(ctx, dbString, false)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer func() {
		_ = closers.Close()
	}()

	if err := Migrate("pgx", dbString, logrus.NewEntry(log)); err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}

	repository = New(db, logrus.NewEntry(log)).(*repo)

	code := m.Run()

	cleanup()

	os.Exit(code)
}

func newTestRepo(t testing.TB, stmts ...string) Repo {
	t.Helper()
	newRepo, tx, err := repository.WithTx(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	txRepo := &transactionRepo{
		Repo: newRepo,
		tx:   tx,
	}

	for _, s := range stmts {
		_, err := tx.Exec(context.Background(), s)
		if err != nil {
			t.Fatalf("Error executing:\n%v\nErr: %v", s, err)
		}
	}
	return txRepo
}

type transactionRepo struct {
	Repo
	tx pgx.Tx
}

func (r *transactionRepo) Close() {
	_ = r.tx.Rollback(context.Background())
}
