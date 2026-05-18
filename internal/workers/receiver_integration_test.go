//go:build integration_test

package workers_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/nais/fasit/internal/testinfra"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
)

func TestReceiverIntegration(t *testing.T) {
	ctx := context.Background()
	db := testinfra.Start(ctx, t)

	tenantID := uuid.New()
	envID := uuid.New()
	fixtures := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'mytenant')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, tenant_id, name, kind) VALUES ('%s', '%s', 'dev', 'tenant')", envID, tenantID),
	}

	t.Run("releaseStatus: stores releases", func(t *testing.T) {
		pool := db.Pool(ctx, t)
		ctx := testinfra.Context(ctx, pool)
		testinfra.Exec(ctx, t, pool, fixtures...)

		repo := testinfra.Repo(pool)

		releases := message.HelmRelease{
			Releases: []message.Release{
				{Name: "app1", Status: "deployed", Version: "1.0.0"},
				{Name: "app2", Status: "deployed", Version: "2.0.0"},
			},
		}
		data, _ := json.Marshal(releases)

		rec := workers.NewReceiver(
			&fakeClient{messages: []message.Status{
				{Type: message.StatusTypeHelmReleases, Tenant: "mytenant", Environment: "dev", Data: data},
			}},
			repo,
			logrus.NewEntry(logrus.New()),
			fake.NewFakeSlackClient(),
			"test",
		)
		rec.Run(ctx)

		got, err := repo.ReleaseStatusesGet(ctx, envID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d releases, want 2", len(got))
		}
	})

	t.Run("releaseStatus: replaces on second message", func(t *testing.T) {
		pool := db.Pool(ctx, t)
		ctx := testinfra.Context(ctx, pool)
		testinfra.Exec(ctx, t, pool, fixtures...)

		repo := testinfra.Repo(pool)

		send := func(releases []message.Release) {
			data, _ := json.Marshal(message.HelmRelease{Releases: releases})
			rec := workers.NewReceiver(
				&fakeClient{messages: []message.Status{
					{Type: message.StatusTypeHelmReleases, Tenant: "mytenant", Environment: "dev", Data: data},
				}},
				repo,
				logrus.NewEntry(logrus.New()),
				fake.NewFakeSlackClient(),
				"test",
			)
			rec.Run(ctx)
		}

		send([]message.Release{
			{Name: "old-app", Status: "deployed", Version: "1.0.0"},
		})
		send([]message.Release{
			{Name: "new-app", Status: "deployed", Version: "2.0.0"},
		})

		got, err := repo.ReleaseStatusesGet(ctx, envID)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Fatalf("got %d releases, want 1 (old should be replaced)", len(got))
		}
		if got[0].Name != "new-app" {
			t.Errorf("release name = %q, want %q", got[0].Name, "new-app")
		}
	})

	t.Run("healthStatus: stores reported_at", func(t *testing.T) {
		pool := db.Pool(ctx, t)
		ctx := testinfra.Context(ctx, pool)
		testinfra.Exec(ctx, t, pool, fixtures...)

		repo := testinfra.Repo(pool)
		reportedAt := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		data, _ := json.Marshal(message.Health{ReportedAt: reportedAt})
		rec := workers.NewReceiver(
			&fakeClient{messages: []message.Status{
				{Type: message.StatusTypeHealth, Tenant: "mytenant", Environment: "dev", Data: data},
			}},
			repo,
			logrus.NewEntry(logrus.New()),
			fake.NewFakeSlackClient(),
			"test",
		)
		rec.Run(ctx)

		got, err := naisdstatus.Get(ctx, envID)
		if err != nil {
			t.Fatal(err)
		}
		if !got.ReportedAt.Equal(reportedAt) {
			t.Errorf("ReportedAt = %v, want %v", got.ReportedAt, reportedAt)
		}
	})

	t.Run("helmStatus: updates deploy instruction status", func(t *testing.T) {
		pool := db.Pool(ctx, t)
		ctx := testinfra.Context(ctx, pool)
		testinfra.Exec(ctx, t, pool, fixtures...)

		diID := uuid.New()
		testinfra.Exec(ctx, t, pool,
			fmt.Sprintf("INSERT INTO deploy_instructions (id, environment_id, feature_name, feature_version, hash) VALUES ('%s', '%s', 'myfeature', '1.0.0', 'abc')", diID, envID),
		)

		repo := testinfra.Repo(pool)
		helm := map[string]any{
			"name":          "myfeature",
			"rolloutStatus": "deployed",
			"version":       "1.0.0",
			"DIID":          diID.String(),
		}
		data, _ := json.Marshal(helm)

		rec := workers.NewReceiver(
			&fakeClient{messages: []message.Status{
				{Type: message.StatusTypeHelm, Tenant: "mytenant", Environment: "dev", Data: data},
			}},
			repo,
			logrus.NewEntry(logrus.New()),
			fake.NewFakeSlackClient(),
			"test",
		)
		rec.Run(ctx)

		// Verify the deploy instruction status was updated.
		var status string
		err := pool.QueryRow(ctx, "SELECT status FROM deploy_instructions WHERE id = $1", diID).Scan(&status)
		if err != nil {
			t.Fatal(err)
		}
		if status != "deployed" {
			t.Errorf("status = %q, want %q", status, "deployed")
		}
	})
}

type fakeClient struct {
	messages []message.Status
}

func (f *fakeClient) Receive(ctx context.Context, fn func(context.Context, message.Status) error) error {
	for _, m := range f.messages {
		if err := fn(ctx, m); err != nil {
			return err
		}
	}
	return nil
}
