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
		// When limit is 0 or negative, it defaults to 50; offset=0 is already the default
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 0, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should return all 10 records (less than default limit of 50)
		if len(got.Items) != 10 {
			t.Errorf("expected 10 records with default limit, got %d", len(got.Items))
		}

		// Should be ordered by last_modified DESC (newest first)
		if got.Items[0].Version != "1.30.9" {
			t.Errorf("expected first record to be 1.30.9 (newest), got %s", got.Items[0].Version)
		}
		if got.Items[9].Version != "1.30.0" {
			t.Errorf("expected last record to be 1.30.0 (oldest), got %s", got.Items[9].Version)
		}

		// Check pagination metadata
		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (all records returned)")
		}
	})

	t.Run("negative limit normalizes to default", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, -1, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should still return all records (negative limit becomes 50)
		if len(got.Items) != 10 {
			t.Errorf("expected 10 records with negative limit, got %d", len(got.Items))
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false")
		}
	})

	t.Run("negative offset normalizes to 0", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 5, -1)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should return 5 records starting from beginning (offset becomes 0)
		if len(got.Items) != 5 {
			t.Errorf("expected 5 records, got %d", len(got.Items))
		}
		if got.Items[0].Version != "1.30.9" {
			t.Errorf("expected first record to be 1.30.9, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (5 items returned, 10 total)")
		}
	})

	t.Run("limit controls number of records returned", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 3, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got.Items) != 3 {
			t.Errorf("expected 3 records with limit=3, got %d", len(got.Items))
		}

		// Should be the 3 newest records
		if got.Items[0].Version != "1.30.9" {
			t.Errorf("expected first record to be 1.30.9, got %s", got.Items[0].Version)
		}
		if got.Items[2].Version != "1.30.7" {
			t.Errorf("expected third record to be 1.30.7, got %s", got.Items[2].Version)
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (3 items returned, 10 total)")
		}
	})

	t.Run("offset skips records correctly", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 3, 2)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got.Items) != 3 {
			t.Errorf("expected 3 records, got %d", len(got.Items))
		}

		// With offset=2, should skip the 2 newest and return next 3
		// Order: 1.30.9 (skip), 1.30.8 (skip), 1.30.7, 1.30.6, 1.30.5
		if got.Items[0].Version != "1.30.7" {
			t.Errorf("expected first record to be 1.30.7, got %s", got.Items[0].Version)
		}
		if got.Items[2].Version != "1.30.5" {
			t.Errorf("expected third record to be 1.30.5, got %s", got.Items[2].Version)
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (offset 2 + 3 items = 5 < 10 total)")
		}
	})

	t.Run("offset beyond available records returns empty", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 5, 20)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		if len(got.Items) != 0 {
			t.Errorf("expected 0 records with offset beyond available, got %d", len(got.Items))
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (offset beyond all records)")
		}
	})

	t.Run("limit larger than available records", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 100, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should return all 10 available records
		if len(got.Items) != 10 {
			t.Errorf("expected 10 records (all available), got %d", len(got.Items))
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (all records returned)")
		}
	})

	t.Run("limit exceeding maximum is capped at 1000", func(t *testing.T) {
		// Try to request an extremely large limit
		got, err := repo.ClusterUpgradeHistoryGet(ctx, tenantID, envID, 2147483647, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGet() error = %v", err)
		}

		// Should still return all 10 records (capped at 1000, but only 10 exist)
		if len(got.Items) != 10 {
			t.Errorf("expected 10 records (all available), got %d", len(got.Items))
		}

		if got.TotalCount != 10 {
			t.Errorf("expected TotalCount 10, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (all records returned)")
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

			if len(page.Items) == 0 {
				break
			}

			for _, upgrade := range page.Items {
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

		if len(got.Items) != 0 {
			t.Errorf("expected empty result, got %d records", len(got.Items))
		}

		if got.TotalCount != 0 {
			t.Errorf("expected TotalCount 0, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (no records)")
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

	if len(got.Items) != 1 {
		t.Fatalf("expected 1 record, got %d", len(got.Items))
	}

	upgrade := got.Items[0]

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

	// Check pagination metadata
	if got.TotalCount != 1 {
		t.Errorf("expected TotalCount 1, got %d", got.TotalCount)
	}
	if got.HasMore {
		t.Error("expected HasMore to be false (only 1 record)")
	}
}

func TestRepo_ClusterUpgradeHistoryGetByTenant_Pagination(t *testing.T) {
	tenant1ID := uuid.New()
	tenant2ID := uuid.New()
	env1ID := uuid.New()
	env2ID := uuid.New()
	env3ID := uuid.New()

	// Setup test data: 2 tenants with multiple environments
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'tenant-1')", tenant1ID),
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'tenant-2')", tenant2ID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'env-1', '%s', 'tenant')", env1ID, tenant1ID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'env-2', '%s', 'tenant')", env2ID, tenant1ID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'env-3', '%s', 'tenant')", env3ID, tenant2ID),
	}

	// Create upgrades: 7 for tenant1 (3 in env1, 4 in env2), 3 for tenant2
	baseTime := time.Now().Add(-10 * time.Hour)
	for i := 0; i < 3; i++ {
		upgradeID := uuid.New()
		version := fmt.Sprintf("1.29.%d", i)
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		qs = append(qs, fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, last_modified) VALUES ('%s', '%s', '%s', '%s', 'DONE', '%s', '%s')",
			upgradeID, tenant1ID, env1ID, version, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339),
		))
	}
	for i := 3; i < 7; i++ {
		upgradeID := uuid.New()
		version := fmt.Sprintf("1.28.%d", i)
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		qs = append(qs, fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, last_modified) VALUES ('%s', '%s', '%s', '%s', 'DONE', '%s', '%s')",
			upgradeID, tenant1ID, env2ID, version, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339),
		))
	}
	for i := 7; i < 10; i++ {
		upgradeID := uuid.New()
		version := fmt.Sprintf("1.30.%d", i)
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		qs = append(qs, fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, last_modified) VALUES ('%s', '%s', '%s', '%s', 'DONE', '%s', '%s')",
			upgradeID, tenant2ID, env3ID, version, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339),
		))
	}

	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := context.Background()

	t.Run("returns all upgrades for tenant across all environments", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetByTenant(ctx, tenant1ID, 50, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetByTenant() error = %v", err)
		}

		if len(got.Items) != 7 {
			t.Errorf("expected 7 records for tenant1, got %d", len(got.Items))
		}

		// Verify order (DESC by last_modified) - newest first
		if got.Items[0].Version != "1.28.6" {
			t.Errorf("expected first record to be 1.28.6, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 7 {
			t.Errorf("expected TotalCount 7, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (all records returned)")
		}
	})

	t.Run("limit parameter works", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetByTenant(ctx, tenant1ID, 3, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetByTenant() error = %v", err)
		}

		if len(got.Items) != 3 {
			t.Errorf("expected 3 records, got %d", len(got.Items))
		}

		if got.TotalCount != 7 {
			t.Errorf("expected TotalCount 7, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (3 items returned, 7 total)")
		}
	})

	t.Run("offset parameter works", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetByTenant(ctx, tenant1ID, 3, 2)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetByTenant() error = %v", err)
		}

		if len(got.Items) != 3 {
			t.Errorf("expected 3 records, got %d", len(got.Items))
		}

		// Should skip first 2 records
		if got.Items[0].Version != "1.28.4" {
			t.Errorf("expected first record after offset to be 1.28.4, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 7 {
			t.Errorf("expected TotalCount 7, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (offset 2 + 3 items = 5 < 7 total)")
		}
	})

	t.Run("negative limit defaults to 50", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetByTenant(ctx, tenant1ID, -1, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetByTenant() error = %v", err)
		}

		// Should return all 7 records (less than default 50)
		if len(got.Items) != 7 {
			t.Errorf("expected 7 records with default limit, got %d", len(got.Items))
		}

		if got.TotalCount != 7 {
			t.Errorf("expected TotalCount 7, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false")
		}
	})

	t.Run("negative offset defaults to 0", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetByTenant(ctx, tenant1ID, 2, -5)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetByTenant() error = %v", err)
		}

		if len(got.Items) != 2 {
			t.Errorf("expected 2 records, got %d", len(got.Items))
		}

		// Should start from beginning (no offset)
		if got.Items[0].Version != "1.28.6" {
			t.Errorf("expected first record to be 1.28.6, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 7 {
			t.Errorf("expected TotalCount 7, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (2 items returned, 7 total)")
		}
	})

	t.Run("limit exceeding 1000 is capped", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetByTenant(ctx, tenant1ID, 2000, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetByTenant() error = %v", err)
		}

		// Should return all available records (limited by data, not by cap)
		if len(got.Items) != 7 {
			t.Errorf("expected 7 records, got %d", len(got.Items))
		}

		if got.TotalCount != 7 {
			t.Errorf("expected TotalCount 7, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false")
		}
	})
}

func TestRepo_ClusterUpgradeHistoryGetAll_Pagination(t *testing.T) {
	tenant1ID := uuid.New()
	tenant2ID := uuid.New()
	env1ID := uuid.New()
	env2ID := uuid.New()

	// Setup test data: 2 tenants with environments
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'tenant-1')", tenant1ID),
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'tenant-2')", tenant2ID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'env-1', '%s', 'tenant')", env1ID, tenant1ID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'env-2', '%s', 'tenant')", env2ID, tenant2ID),
	}

	// Create 15 total upgrades: 10 for tenant1, 5 for tenant2
	baseTime := time.Now().Add(-15 * time.Hour)
	for i := 0; i < 10; i++ {
		upgradeID := uuid.New()
		version := fmt.Sprintf("1.29.%d", i)
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		qs = append(qs, fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, last_modified) VALUES ('%s', '%s', '%s', '%s', 'DONE', '%s', '%s')",
			upgradeID, tenant1ID, env1ID, version, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339),
		))
	}
	for i := 10; i < 15; i++ {
		upgradeID := uuid.New()
		version := fmt.Sprintf("1.30.%d", i)
		timestamp := baseTime.Add(time.Duration(i) * time.Hour)
		qs = append(qs, fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, start_time, last_modified) VALUES ('%s', '%s', '%s', '%s', 'DONE', '%s', '%s')",
			upgradeID, tenant2ID, env2ID, version, timestamp.Format(time.RFC3339), timestamp.Format(time.RFC3339),
		))
	}

	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := context.Background()

	t.Run("returns all upgrades across all tenants", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, 50, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		if len(got.Items) != 15 {
			t.Errorf("expected 15 records total, got %d", len(got.Items))
		}

		// Verify order (DESC by last_modified) - newest first
		if got.Items[0].Version != "1.30.14" {
			t.Errorf("expected first record to be 1.30.14, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (all records returned)")
		}
	})

	t.Run("limit parameter works", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, 5, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		if len(got.Items) != 5 {
			t.Errorf("expected 5 records, got %d", len(got.Items))
		}

		// Should be 5 newest
		if got.Items[0].Version != "1.30.14" {
			t.Errorf("expected first record to be 1.30.14, got %s", got.Items[0].Version)
		}
		if got.Items[4].Version != "1.30.10" {
			t.Errorf("expected fifth record to be 1.30.10, got %s", got.Items[4].Version)
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (5 items returned, 15 total)")
		}
	})

	t.Run("offset parameter works", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, 3, 5)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		if len(got.Items) != 3 {
			t.Errorf("expected 3 records, got %d", len(got.Items))
		}

		// Should skip first 5, return next 3
		if got.Items[0].Version != "1.29.9" {
			t.Errorf("expected first record after offset to be 1.29.9, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (offset 5 + 3 items = 8 < 15 total)")
		}
	})

	t.Run("negative limit defaults to 50", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, -1, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		// Should return all 15 records (less than default 50)
		if len(got.Items) != 15 {
			t.Errorf("expected 15 records with default limit, got %d", len(got.Items))
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false")
		}
	})

	t.Run("negative offset defaults to 0", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, 3, -10)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		if len(got.Items) != 3 {
			t.Errorf("expected 3 records, got %d", len(got.Items))
		}

		// Should start from beginning
		if got.Items[0].Version != "1.30.14" {
			t.Errorf("expected first record to be 1.30.14, got %s", got.Items[0].Version)
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if !got.HasMore {
			t.Error("expected HasMore to be true (3 items returned, 15 total)")
		}
	})

	t.Run("limit exceeding 1000 is capped", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, 5000, 0)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		// Should return all available records
		if len(got.Items) != 15 {
			t.Errorf("expected 15 records, got %d", len(got.Items))
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false")
		}
	})

	t.Run("offset beyond records returns empty", func(t *testing.T) {
		got, err := repo.ClusterUpgradeHistoryGetAll(ctx, 10, 20)
		if err != nil {
			t.Fatalf("ClusterUpgradeHistoryGetAll() error = %v", err)
		}

		if len(got.Items) != 0 {
			t.Errorf("expected 0 records with offset beyond available, got %d", len(got.Items))
		}

		if got.TotalCount != 15 {
			t.Errorf("expected TotalCount 15, got %d", got.TotalCount)
		}
		if got.HasMore {
			t.Error("expected HasMore to be false (offset beyond all records)")
		}
	})
}

func TestRepo_ClusterUpgradeBypassDelay(t *testing.T) {
	tenantID := uuid.New()

	// Use different environment IDs to avoid unique constraint violation
	// (only one in-progress upgrade per environment allowed)
	waitingEnvID := uuid.New()
	createdEnvID := uuid.New()
	doneEnvID := uuid.New()
	failedEnvID := uuid.New()

	// Setup test data with upgrades in different statuses
	waitingUpgradeID := uuid.New()
	createdUpgradeID := uuid.New()
	doneUpgradeID := uuid.New()
	failedUpgradeID := uuid.New()

	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test-tenant')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'waiting-env', '%s', 'tenant')", waitingEnvID, tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'created-env', '%s', 'tenant')", createdEnvID, tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'done-env', '%s', 'tenant')", doneEnvID, tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, tenant_id, kind) VALUES ('%s', 'failed-env', '%s', 'tenant')", failedEnvID, tenantID),
		fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, is_automatic, start_time, last_modified) VALUES ('%s', '%s', '%s', '1.30.0', 'WAITING', true, '%s', '%s')",
			waitingUpgradeID, tenantID, waitingEnvID, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
		),
		fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, is_automatic, start_time, last_modified) VALUES ('%s', '%s', '%s', '1.30.1', 'CREATED', true, '%s', '%s')",
			createdUpgradeID, tenantID, createdEnvID, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
		),
		fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, is_automatic, start_time, last_modified) VALUES ('%s', '%s', '%s', '1.30.2', 'DONE', true, '%s', '%s')",
			doneUpgradeID, tenantID, doneEnvID, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
		),
		fmt.Sprintf(
			"INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, is_automatic, start_time, last_modified) VALUES ('%s', '%s', '%s', '1.30.3', 'FAILED', true, '%s', '%s')",
			failedUpgradeID, tenantID, failedEnvID, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339),
		),
	}

	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := context.Background()

	t.Run("successfully bypass WAITING upgrade", func(t *testing.T) {
		upgrade, err := repo.ClusterUpgradeBypassDelay(ctx, waitingUpgradeID)
		if err != nil {
			t.Fatalf("ClusterUpgradeBypassDelay() error = %v", err)
		}

		// Verify the returned upgrade has the correct status
		if upgrade.UpgradeStatus != model.UpgradeStatusCreated {
			t.Errorf("expected status to be CREATED, got %s", upgrade.UpgradeStatus)
		}

		if upgrade.IsAutomatic == nil || *upgrade.IsAutomatic {
			t.Error("expected is_automatic to be false after bypass")
		}
	})

	t.Run("reject bypass of CREATED upgrade", func(t *testing.T) {
		upgrade, err := repo.ClusterUpgradeBypassDelay(ctx, createdUpgradeID)
		if err == nil {
			t.Fatal("expected error when bypassing CREATED upgrade, got nil")
		}
		if upgrade != nil {
			t.Error("expected nil upgrade when error occurs")
		}

		expectedMsg := "cannot bypass delay: upgrade is in 'CREATED' status; only upgrades in 'WAITING' status can have their delay bypassed"
		if err.Error() != expectedMsg {
			t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("reject bypass of DONE upgrade", func(t *testing.T) {
		upgrade, err := repo.ClusterUpgradeBypassDelay(ctx, doneUpgradeID)
		if err == nil {
			t.Fatal("expected error when bypassing DONE upgrade, got nil")
		}
		if upgrade != nil {
			t.Error("expected nil upgrade when error occurs")
		}

		expectedMsg := "cannot bypass delay: upgrade is in 'DONE' status; only upgrades in 'WAITING' status can have their delay bypassed"
		if err.Error() != expectedMsg {
			t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("reject bypass of FAILED upgrade", func(t *testing.T) {
		upgrade, err := repo.ClusterUpgradeBypassDelay(ctx, failedUpgradeID)
		if err == nil {
			t.Fatal("expected error when bypassing FAILED upgrade, got nil")
		}
		if upgrade != nil {
			t.Error("expected nil upgrade when error occurs")
		}

		expectedMsg := "cannot bypass delay: upgrade is in 'FAILED' status; only upgrades in 'WAITING' status can have their delay bypassed"
		if err.Error() != expectedMsg {
			t.Errorf("expected error message %q, got %q", expectedMsg, err.Error())
		}
	})

	t.Run("handle non-existent upgrade ID", func(t *testing.T) {
		nonExistentID := uuid.New()
		upgrade, err := repo.ClusterUpgradeBypassDelay(ctx, nonExistentID)
		if err == nil {
			t.Fatal("expected error for non-existent upgrade ID, got nil")
		}
		if upgrade != nil {
			t.Error("expected nil upgrade when error occurs")
		}
		// The error should come from ClusterUpgradesGetByID when the upgrade doesn't exist
	})
}
