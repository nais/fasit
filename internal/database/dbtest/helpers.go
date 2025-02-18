package dbtest

import (
	"context"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	_ "github.com/lib/pq"

	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

func DockerSQLPool() (string, func()) {
	var dbString string
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

	_ = resource.Expire(120) // Tell docker to hard kill the container in 120 seconds

	// exponential backoff-retry, because the application in the container might not be ready to accept connections yet
	if err := pool.Retry(func() error {
		ctx := context.Background()
		dbString = "user=postgres dbname=fasit sslmode=disable password=postgres host=localhost port=" + resource.GetPort("5432/tcp")
		conn, err := pgx.Connect(ctx, dbString)
		if err != nil {
			return err
		}
		return conn.Ping(ctx)
	}); err != nil {
		log.Fatalf("Could not connect to docker: %s", err)
	}

	return dbString, func() {
		// You can't defer this because os.Exit doesn't care for defer
		if err := pool.Purge(resource); err != nil {
			log.Fatalf("Could not purge resource: %s", err)
		}
	}
}
