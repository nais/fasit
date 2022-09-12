//go:build integration_test

package database

import (
	"io"
	"os"
	"testing"

	_ "github.com/lib/pq"
	"github.com/nais/fasit/pkg/database/dbtest"

	"github.com/DATA-DOG/go-txdb"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var dbString = ""

func TestMain(m *testing.M) {
	db, dbs, cleanup := dbtest.DockerSQLPool()
	dbString = dbs

	log := logrus.New()
	log.Out = io.Discard
	if err := Migrate(db, logrus.NewEntry(log)); err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}
	txdb.Register("txdb", "postgres", dbString)

	code := m.Run()

	cleanup()

	os.Exit(code)
}

func newTestRepo(t testing.TB, stmts ...string) Repo {
	t.Helper()

	db, err := NewDB("txdb", uuid.NewString())
	if err != nil {
		t.Fatalf("Could not create db: %v", err)
	}

	r := New(db, uuid.NewString(), newTestLogger())
	for _, s := range stmts {
		_, err := r.(*repo).db.Exec(s)
		if err != nil {
			t.Fatalf("Error executing:\n%v\nErr: %v", s, err)
		}
	}
	return r
}

func newTestLogger() *logrus.Entry {
	log := logrus.New()
	if !testing.Verbose() {
		log.Out = io.Discard
	}
	return logrus.NewEntry(log)
}
