//go:build integration_test

package database

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
)

func TestRepo_ClusterUpgradeHistoryGet_Pagination(t *testing.T) {
	tenantID := uuid.New()
	envID := uuid.New()

	// Setup test data: create 10 cluster upgrades with different timestamps
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test-tenant')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'test-env', '%s', 'tenant')", envID, tenantID),
	}

	// Create 10 upgrades with sequential versions and timestamps
	baseTime := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 10; i++ {
		upgradeID := uuid.New()
		version := fmt.Sprintf("1.30.%d", i)
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)

		qs = append(qs, fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, last_modified) VALUES ('%s', '%s', '%s', '%s', 'DONE', '%s', '%s')",
			upgradeID, tenantID, envID, version, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339),
		))
	}

	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := context.Background()

	t.Run("default values - limit 50, offset 0", func(t *testing.T) {
		// When limit and offset are 0, defaults should apply
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 0, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should return all 10 records (less than default limit of 50)
		if len(got) != 10 {
			t.Errorf("expected 10 records with default limit, got %d", len(got))
		}

		// Should be ordered by last_modified DESC (newest first)
		if got[0].Version != "1.30.9" {
			t.Errorf("expected first record to be 1.30.9 (newest), got %s", got[0].Version)
		}
		if got[9].Version != "1.30.0" {
			t.Errorf("expected last record to be 1.30.0 (oldest), got %s", got[9].Version)
		}
	})

	t.Run("negative limit normalizes to default", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, -1, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should still return all records (negative limit becomes 50)
		if len(got) != 10 {
			t.Errorf("expected 10 records with negative limit, got %d", len(got))
		}
	})

	t.Run("negative offset normalizes to 0", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 5, -1)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should return 5 records starting from beginning (offset becomes 0)
		if len(got) != 5 {
			t.Errorf("expected 5 records, got %d", len(got))
		}
		if got[0].Version != "1.30.9" {
			t.Errorf("expected first record to be 1.30.9, got %s", got[0].Version)
		}
	})

	t.Run("limit controls number of records returned", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 3, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got) != 3 {
			t.Errorf("expected 3 records with limit=3, got %d", len(got))
		}

		// Should be the 3 newest records
		if got[0].Version != "1.30.9" {
			t.Errorf("expected first record to be 1.30.9, got %s", got[0].Version)
		}
		if got[2].Version != "1.30.7" {
			t.Errorf("expected third record to be 1.30.7, got %s", got[2].Version)
		}
	})

	t.Run("offset skips records correctly", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 3, 2)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got) != 3 {
			t.Errorf("expected 3 records, got %d", len(got))
		}

		// With offset=2, should skip the 2 newest and return next 3
		// Order: 1.30.9 (skip), 1.30.8 (skip), 1.30.7, 1.30.6, 1.30.5
		if got[0].Version != "1.30.7" {
			t.Errorf("expected first record to be 1.30.7, got %s", got[0].Version)
		}
		if got[2].Version != "1.30.5" {
			t.Errorf("expected third record to be 1.30.5, got %s", got[2].Version)
		}
	})

	t.Run("offset beyond available records returns empty", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 5, 20)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got) != 0 {
			t.Errorf("expected 0 records with offset beyond available, got %d", len(got))
		}
	})

	t.Run("limit larger than available records", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 100, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should return all 10 available records
		if len(got) != 10 {
			t.Errorf("expected 10 records (all available), got %d", len(got))
		}
	})

	t.Run("pagination through all records", func(t *testing.T) {
		// Fetch in pages of 3
		var allVersions []string
		offset := int32(0)
		pageSize := int32(3)

		for {
			page, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, pageSize, offset)
			if err != nil {
				t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
			}

			if len(page) == 0 {
				break
			}

			for _, upgrade := range page {
				allVersions = append(allVersions, upgrade.Version)
			}

			offset += pageSize
		}

		// Should have collected all 10 versions
		if len(allVersions) != 10 {
			t.Errorf("expected to paginate through 10 records, got %d", len(allVersions))
		}

		// Should be in DESC order (newest to oldest)
		expectedVersions := []string{"1.30.9", "1.30.8", "1.30.7", "1.30.6", "1.30.5", "1.30.4", "1.30.3", "1.30.2", "1.30.1", "1.30.0"}
		for i, expected := range expectedVersions {
			if allVersions[i] != expected {
				t.Errorf("at position %d: expected %s, got %s", i, expected, allVersions[i])
			}
		}
	})
}

func TestRepo_ClusterUpgradeHistoryGet_EmptyResult(t *testing.T) {
	tenantID := uuid.New()
	envID := uuid.New()

	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test-tenant')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'test-env', '%s', 'tenant')", envID, tenantID),
	}

	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := context.Background()

	t.Run("empty history returns empty list", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 10, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got) != 0 {
			t.Errorf("expected empty result, got %d records", len(got))
		}
	})
}

func TestRepo_ClusterUpgradeHistoryGet_ReturnedFields(t *testing.T) {
	tenantID := uuid.New()
	envID := uuid.New()
	upgradeID := uuid.New()

	startTime := time.Now().Add(-2 * time.Hour)
	upgradeStartTime := time.Now().Add(-1 * time.Hour)
	lastModified := time.Now()

	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test-tenant')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'test-env', '%s', 'tenant')", envID, tenantID),
		fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, upgrade_start_time, last_modified, is_automatic, slack_message_timestamp, slack_channel_id) VALUES ('%s', '%s', '%s', '1.30.0', 'DONE', '%s', '%s', '%s', true, 'ts123', 'ch456')",
			upgradeID, tenantID, envID,
			startTime.Format(time.RFC3339),
			upgradeStartTime.Format(time.RFC3339),
			lastModified.Format(time.RFC3339),
		),
	}

	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := context.Background()

	got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 10, 0)
	if err != nil {
		t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got))
	}

	upgrade := got[0]

	// Verify all fields are populated correctly
	if upgrade.ID != upgradeID {
		t.Errorf("expected ID %s, got %s", upgradeID, upgrade.ID)
	}
	if upgrade.Version != "1.30.0" {
		t.Errorf("expected version 1.30.0, got %s", upgrade.Version)
	}
	if upgrade.UpgradeStatus != model.UpgradeStatusDone {
		t.Errorf("expected status DONE, got %s", upgrade.UpgradeStatus)
	}
	if upgrade.EnvironmentID != envID {
		t.Errorf("expected environment ID %s, got %s", envID, upgrade.EnvironmentID)
	}
	if upgrade.IsAutomatic == nil || !*upgrade.IsAutomatic {
		t.Error("expected IsAutomatic to be true")
	}
	if upgrade.SlackMessageTimestamp != "ts123" {
		t.Errorf("expected SlackMessageTimestamp ts123, got %s", upgrade.SlackMessageTimestamp)
	}
	if upgrade.SlackChannelID != "ch456" {
		t.Errorf("expected SlackChannelID ch456, got %s", upgrade.SlackChannelID)
	}
	if upgrade.UpgradeStartTime == nil {
		t.Error("expected UpgradeStartTime to be set")
	}
}
