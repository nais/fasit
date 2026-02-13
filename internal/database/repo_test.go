//go:build integration_test

package database

import (
	"context"
	"io"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/nais/fasit/internal/database/dbtest"
	"github.com/sirupsen/logrus"
)

var (
	dbString   = ""
	repository *repo
	pool       *pgxpool.Pool
)

func TestMain(m *testing.M) {
	dbs, cleanup := dbtest.DockerSQLPool(context.Background())
	dbString = dbs

	log := logrus.New()
	log.Out = io.Discard
	ctx := context.Background()

	p, closers, err := NewConnPool(ctx, dbString, log)
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	defer func() {
		_ = closers.Close()
	}()
	pool = p

	repository = NewRepo(pool, logrus.NewEntry(log)).(*repo)

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
