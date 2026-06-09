//go:build integration_test

package reconciler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/nais/fasit/internal/testinfra"
	"go.opentelemetry.io/otel/metric/noop"
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

		releases := message.HelmRelease{
			Releases: []message.Release{
				{Name: "app1", Status: "deployed", Version: "1.0.0"},
				{Name: "app2", Status: "deployed", Version: "2.0.0"},
			},
		}
		data, _ := json.Marshal(releases)

		rec := reconciler.NewReceiver(pool, &fakeClient{messages: []message.Status{
			{Type: message.StatusTypeHelmReleases, Tenant: "mytenant", Environment: "dev", Data: data},
		}}, slog.Default(), fake.NewFakeSlackClient(), "test", noop.Meter{})
		rec.Run(ctx)

		got, err := naisdstatus.ListReleaseStatuses(ctx, envID)
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

		send := func(releases []message.Release) {
			data, _ := json.Marshal(message.HelmRelease{Releases: releases})
			rec := reconciler.NewReceiver(pool, &fakeClient{messages: []message.Status{
				{Type: message.StatusTypeHelmReleases, Tenant: "mytenant", Environment: "dev", Data: data},
			}}, slog.Default(), fake.NewFakeSlackClient(), "test", noop.Meter{})
			rec.Run(ctx)
		}

		send([]message.Release{
			{Name: "old-app", Status: "deployed", Version: "1.0.0"},
		})
		send([]message.Release{
			{Name: "new-app", Status: "deployed", Version: "2.0.0"},
		})

		got, err := naisdstatus.ListReleaseStatuses(ctx, envID)
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

		reportedAt := time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)

		data, _ := json.Marshal(message.Health{ReportedAt: reportedAt})
		rec := reconciler.NewReceiver(pool, &fakeClient{messages: []message.Status{
			{Type: message.StatusTypeHealth, Tenant: "mytenant", Environment: "dev", Data: data},
		}}, slog.Default(), fake.NewFakeSlackClient(), "test", noop.Meter{})
		rec.Run(ctx)

		got, err := naisdstatus.Get(ctx, envID)
		if err != nil {
			t.Fatal(err)
		}
		if !got.ReportedAt.Equal(reportedAt) {
			t.Errorf("ReportedAt = %v, want %v", got.ReportedAt, reportedAt)
		}
	})

	t.Run("helmStatus: appends terminal deploy_log row", func(t *testing.T) {
		pool := db.Pool(ctx, t)
		ctx := testinfra.Context(ctx, pool)
		testinfra.Exec(ctx, t, pool, fixtures...)

		faID := uuid.New()
		diID := uuid.New()
		testinfra.Exec(ctx, t, pool,
			"INSERT INTO feature_data (name, version, values, chart, source, description, kinds, default_values, dependencies) VALUES ('myfeature', '1.0.0', '{}', 'oci://chart', 'source', 'description', '{tenant}', '{}', '{}')",
			fmt.Sprintf("INSERT INTO feature_assignments (id, feature_name, version) VALUES ('%s', 'myfeature', '1.0.0')", faID),
			fmt.Sprintf("INSERT INTO deploy_log (diid, environment_id, feature_assignment_id, feature_name, feature_version, status, hash) VALUES ('%s', '%s', '%s', 'myfeature', '1.0.0', 'sent', 'abc')", diID, envID, faID),
		)

		helm := map[string]any{
			"name":          "myfeature",
			"rolloutStatus": "deployed",
			"version":       "1.0.0",
			"DIID":          diID.String(),
		}
		data, _ := json.Marshal(helm)

		rec := reconciler.NewReceiver(pool, &fakeClient{messages: []message.Status{
			{Type: message.StatusTypeHelm, Tenant: "mytenant", Environment: "dev", Data: data},
		}}, slog.Default(), fake.NewFakeSlackClient(), "test", noop.Meter{})
		rec.Run(ctx)

		// Verify the receiver appended a terminal deployed row carrying the hash.
		var status, hash string
		err := pool.QueryRow(ctx, "SELECT status, hash FROM deploy_log WHERE diid = $1 ORDER BY created DESC LIMIT 1", diID).Scan(&status, &hash)
		if err != nil {
			t.Fatal(err)
		}
		if status != "deployed" {
			t.Errorf("status = %q, want %q", status, "deployed")
		}
		if hash != "abc" {
			t.Errorf("hash = %q, want %q (carried forward)", hash, "abc")
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
