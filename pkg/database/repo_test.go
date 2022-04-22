//go:build integration_test

package database

import (
	"database/sql"
	"io"
	"log"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/DATA-DOG/go-txdb"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/sirupsen/logrus"
)

var dbString string

func TestMain(m *testing.M) {
	dockerHost := os.Getenv("HOME") + "/.colima/docker.sock"
	_, err := os.Stat(dockerHost)
	if err != nil {
		// uses a sensible default on windows (tcp/http) and linux/osx (socket)
		dockerHost = ""
	} else {
		dockerHost = "unix://" + dockerHost
	}

	pool, err := dockertest.NewPool(dockerHost)
	if err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	// pulls an image, creates a container based on it and runs it
	resource, err := pool.RunWithOptions(
		&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "14",
			Env:        []string{"POSTGRES_PASSWORD=postgres", "POSTGRES_DB=fasit"},
		},
		func(config *docker.HostConfig) {
			// set AutoRemove to true so that stopped container goes away by itself
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		log.Fatalf("Could not start resource: %s", err)
	}

	resource.Expire(120) // Tell docker to hard kill the container in 120 seconds

	var db *sql.DB
	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		var err error
		dbString = "user=postgres dbname=fasit sslmode=disable password=postgres host=localhost port=" + resource.GetPort("5432/tcp")
		db, err = sql.Open("postgres", dbString)
		if err != nil {
			return err
		}
		return db.Ping()
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	log := logrus.New()
	log.Out = io.Discard
	if err := Migrate(db, logrus.NewEntry(log)); err != nil {
		log.Fatalf("Could not migrate: %v", err)
	}
	txdb.Register("txdb", "postgres", dbString)

	code := m.Run()

	// You can't defer this because os.Exit doesn't care for defer
	if err := pool.Purge(resource); err != nil {
		log.Fatalf("Could not purge resource: %s", err)
	}

	os.Exit(code)
}

func newTestRepo(t testing.TB, name string) Repo {
	t.Helper()

	db, err := NewDB("txdb", name)
	if err != nil {
		t.Fatalf("Could not create db: %v", err)
	}

	return New(db, newTestLogger())
}

func newTestLogger() *logrus.Entry {
	log := logrus.New()
	if !testing.Verbose() {
		log.Out = io.Discard
	}
	return logrus.NewEntry(log)
}
