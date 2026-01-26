package database

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type ClusterUpgraderRepo interface {
	CreateOrUpdateClusterOperation(ctx context.Context, tenantID, envID, versionID uuid.UUID, op *containerpb.Operation) (*model.EnvironmentOperation, error)
	GetActiveClusterOperation(ctx context.Context, tenantID, envID uuid.UUID) (*model.EnvironmentOperation, error)
	CreateClusterUpgrade(ctx context.Context, tenantID, envID uuid.UUID, version string, isAutomatic *bool) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGet(ctx context.Context, tenantID, envID uuid.UUID) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGetByVersion(ctx context.Context, tenantID, envID uuid.UUID, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeHistoryGet(ctx context.Context, tenantID, envID uuid.UUID, limit, offset int32) (*model.ClusterUpgradeHistoryResult, error)
	ClusterUpgradeHistoryGetByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int32) (*model.ClusterUpgradeHistoryResult, error)
	ClusterUpgradeHistoryGetAll(ctx context.Context, limit, offset int32) (*model.ClusterUpgradeHistoryResult, error)
	UpdateClusterUpgradeStatus(ctx context.Context, upgradeID uuid.UUID, status gensql.ClusterUpgradesStatus) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeBypassDelay(ctx context.Context, upgradeID uuid.UUID) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error)
	ClusterOperationsGetByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentOperation, error)
	ClusterOperationsGetByUpgradeID(ctx context.Context, upgradeID uuid.UUID) ([]*model.EnvironmentOperation, error)
	ClusterOperationsGetDanglingForEnvironment(ctx context.Context, tenantID, envID uuid.UUID) (map[uuid.UUID][]*model.EnvironmentOperation, error)
	SetClusterUpgradesSlackMessage(ctx context.Context, id uuid.UUID, slackMessageTS, channelID string) (*model.ClusterUpgradeStatus, error)
}

func (r *repo) ClusterUpgradeHistoryGet(ctx context.Context, tenantID, envID uuid.UUID, limit, offset int32) (*model.ClusterUpgradeHistoryResult, error) {
	// Default to 50 records if limit is not specified or invalid
	if limit <= 0 {
		limit = 50
	}
	// Cap limit at 1000 to prevent abuse
	if limit > 1000 {
		limit = 1000
	}
	// Ensure offset is non-negative
	if offset < 0 {
		offset = 0
	}

	// Get total count
	totalCount, err := r.querier.ClusterUpgradesCountByEnvironmentID(ctx, gensql.ClusterUpgradesCountByEnvironmentIDParams{
		Tenantid: tenantID,
		Envid:    envID,
	})
	if err != nil {
		return nil, err
	}

	clusterUpgrades, err := r.querier.ClusterUpgradesHistoryGetByEnvironmentID(ctx, gensql.ClusterUpgradesHistoryGetByEnvironmentIDParams{
		Tenantid:      tenantID,
		Envid:         envID,
		Historyoffset: offset,
		Historylimit:  limit,
	})
	if err != nil {
		return nil, err
	}

	var upgrades []*model.ClusterUpgradeStatus
	for _, upgrade := range clusterUpgrades {
		upgrades = append(upgrades, clusterUpgradeFromSQL(upgrade))
	}

	hasMore := int64(offset)+int64(len(upgrades)) < totalCount

	return &model.ClusterUpgradeHistoryResult{
		Items:      upgrades,
		TotalCount: int(totalCount), // #nosec G115 -- totalCount unlikely to exceed int max in practice
		HasMore:    hasMore,
	}, nil
}

func (r *repo) ClusterUpgradeHistoryGetByTenant(ctx context.Context, tenantID uuid.UUID, limit, offset int32) (*model.ClusterUpgradeHistoryResult, error) {
	// Default to 50 records if limit is not specified or invalid
	if limit <= 0 {
		limit = 50
	}
	// Cap limit at 1000 to prevent abuse
	if limit > 1000 {
		limit = 1000
	}
	// Ensure offset is non-negative
	if offset < 0 {
		offset = 0
	}

	// Get total count
	totalCount, err := r.querier.ClusterUpgradesCountByTenantID(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	clusterUpgrades, err := r.querier.ClusterUpgradesHistoryGetByTenantID(ctx, gensql.ClusterUpgradesHistoryGetByTenantIDParams{
		Tenantid:      tenantID,
		Historyoffset: offset,
		Historylimit:  limit,
	})
	if err != nil {
		return nil, err
	}

	var upgrades []*model.ClusterUpgradeStatus
	for _, upgrade := range clusterUpgrades {
		upgrades = append(upgrades, clusterUpgradeFromSQL(upgrade))
	}

	hasMore := int64(offset)+int64(len(upgrades)) < totalCount

	return &model.ClusterUpgradeHistoryResult{
		Items:      upgrades,
		TotalCount: int(totalCount), // #nosec G115 -- totalCount unlikely to exceed int max in practice
		HasMore:    hasMore,
	}, nil
}

func (r *repo) ClusterUpgradeHistoryGetAll(ctx context.Context, limit, offset int32) (*model.ClusterUpgradeHistoryResult, error) {
	// Default to 50 records if limit is not specified or invalid
	if limit <= 0 {
		limit = 50
	}
	// Cap limit at 1000 to prevent abuse
	if limit > 1000 {
		limit = 1000
	}
	// Ensure offset is non-negative
	if offset < 0 {
		offset = 0
	}

	// Get total count
	totalCount, err := r.querier.ClusterUpgradesCountAll(ctx)
	if err != nil {
		return nil, err
	}

	clusterUpgrades, err := r.querier.ClusterUpgradesHistoryGetAll(ctx, gensql.ClusterUpgradesHistoryGetAllParams{
		Historyoffset: offset,
		Historylimit:  limit,
	})
	if err != nil {
		return nil, err
	}

	var upgrades []*model.ClusterUpgradeStatus
	for _, upgrade := range clusterUpgrades {
		upgrades = append(upgrades, clusterUpgradeFromSQL(upgrade))
	}

	hasMore := int64(offset)+int64(len(upgrades)) < totalCount

	return &model.ClusterUpgradeHistoryResult{
		Items:      upgrades,
		TotalCount: int(totalCount), // #nosec G115 -- totalCount unlikely to exceed int max in practice
		HasMore:    hasMore,
	}, nil
}

func clusterUpgradeFromSQL(p gensql.ClusterUpgrade) *model.ClusterUpgradeStatus {
	var isAutomatic *bool
	if p.IsAutomatic.Valid {
		isAutomatic = &p.IsAutomatic.Bool
	}

	var upgradeStartTime *time.Time
	if p.UpgradeStartTime.Valid {
		upgradeStartTime = &p.UpgradeStartTime.Time
	}

	return &model.ClusterUpgradeStatus{
		ID:                    p.ID,
		Version:               p.Version,
		UpgradeStatus:         model.UpgradeStatus(p.Status),
		LastModified:          p.LastModified.Time,
		StartTime:             p.StartTime.Time,
		UpgradeStartTime:      upgradeStartTime,
		EnvironmentID:         p.EnvironmentID,
		SlackMessageTimestamp: p.SlackMessageTimestamp.String,
		SlackChannelID:        p.SlackChannelID.String,
		IsAutomatic:           isAutomatic,
	}
}

func clusterOperationFromSQL(p gensql.ClusterOperation) *model.EnvironmentOperation {
	return &model.EnvironmentOperation{
		ID:                  p.ID,
		Name:                p.OperationName,
		Status:              p.Status,
		Type:                p.Type,
		Target:              p.Target,
		Detail:              p.Detail,
		NodesTotal:          int(p.NodesTotal),
		NodesFailed:         int(p.NodesFailed),
		NodesCompleted:      int(p.NodesCompleted),
		NodesDone:           int(p.NodesDone),
		NodePdbDelaySeconds: int(p.NodePdbDelaySeconds),
		StartTime:           p.StartTime.Time,
		LastModified:        p.LastModified.Time,
	}
}

func (r *repo) ClusterOperationsGetByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentOperation, error) {
	clusterOperation, err := r.querier.ClusterOperationsGetByID(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	if clusterOperation.EnvironmentID == uuid.Nil {
		return &model.EnvironmentOperation{}, nil
	}
	return clusterOperationFromSQL(clusterOperation), nil
}

func (r *repo) ClusterOperationsGetByUpgradeID(ctx context.Context, upgradeID uuid.UUID) ([]*model.EnvironmentOperation, error) {
	clusterOperations, err := r.querier.ClusterOperationsGetByUpgradeID(ctx, upgradeID)
	if err != nil {
		return nil, err
	}

	var ops []*model.EnvironmentOperation

	for _, op := range clusterOperations {
		ops = append(ops, clusterOperationFromSQL(op))
	}

	return ops, nil
}

func (r *repo) ClusterOperationsGetDanglingForEnvironment(ctx context.Context, tenantID, envID uuid.UUID) (map[uuid.UUID][]*model.EnvironmentOperation, error) {
	clusterOperations, err := r.querier.ClusterOperationsGetDanglingForEnvironment(ctx, gensql.ClusterOperationsGetDanglingForEnvironmentParams{
		Tenantid: tenantID,
		Envid:    envID,
	})
	if err != nil {
		return nil, err
	}

	// Group operations by upgrade_id for easier processing
	opsByUpgrade := make(map[uuid.UUID][]*model.EnvironmentOperation)

	for _, op := range clusterOperations {
		opsByUpgrade[op.UpgradeID] = append(opsByUpgrade[op.UpgradeID], clusterOperationFromSQL(op))
	}

	return opsByUpgrade, nil
}

func (r *repo) ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesGetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterVersion), nil
}

func (r *repo) SetClusterUpgradesSlackMessage(ctx context.Context, id uuid.UUID, slackMessageTS, channelID string) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrade, err := r.querier.ClusterUpgradesSetSlackMessage(ctx, gensql.ClusterUpgradesSetSlackMessageParams{
		Slackmessagetimestamp: ptrToNullString(&slackMessageTS),
		Slackchannelid:        ptrToNullString(&channelID),
		ID:                    id,
	})
	if err != nil {
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterUpgrade), nil
}

func (r *repo) UpdateClusterUpgradeStatus(ctx context.Context, upgradeID uuid.UUID, status gensql.ClusterUpgradesStatus) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrade, err := r.querier.ClusterUpgradesUpdateStatus(ctx, gensql.ClusterUpgradesUpdateStatusParams{
		Status: status,
		ID:     upgradeID,
	})
	if err != nil {
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterUpgrade), nil
}

func (r *repo) ClusterUpgradeBypassDelay(ctx context.Context, upgradeID uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	// Update both status to CREATED and is_automatic to false
	// This UPDATE will only succeed if the upgrade exists AND is in WAITING status
	updatedUpgrade, err := r.querier.ClusterUpgradesBypassDelay(ctx, upgradeID)
	if err == nil {
		// Success - create audit entry and return
		r.createAudit(ctx, "bypassed upgrade delay", "cluster_upgrades", upgradeID.String())
		return clusterUpgradeFromSQL(updatedUpgrade), nil
	}

	// Handle UPDATE failures
	if !errors.Is(err, pgx.ErrNoRows) {
		// Unexpected error
		return nil, err
	}

	// UPDATE returned no rows - either upgrade doesn't exist or is not in WAITING status
	// Try to get the upgrade to provide a better error message
	upgrade, getErr := r.querier.ClusterUpgradesGetByID(ctx, upgradeID)
	if getErr != nil {
		// Upgrade doesn't exist
		return nil, err
	}

	// Upgrade exists - check its status
	if upgrade.Status != gensql.ClusterUpgradesStatusWAITING {
		return nil, fmt.Errorf("cannot bypass delay: upgrade is in '%s' status; only upgrades in '%s' status can have their delay bypassed", strings.ToUpper(string(upgrade.Status)), "WAITING")
	}

	// This should be impossible: upgrade exists and is in WAITING, but UPDATE returned ErrNoRows
	return nil, fmt.Errorf("internal error: upgrade exists and is in WAITING status, but bypass delay update failed")
}

func (r *repo) ClusterUpgradeGet(ctx context.Context, tenantID, envID uuid.UUID) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrades, err := r.querier.ClusterUpgradesGet(ctx, gensql.ClusterUpgradesGetParams{
		Tenantid: tenantID,
		Envid:    envID,
	})
	if err != nil {
		return nil, err
	}

	if len(clusterUpgrades) == 0 {
		return nil, nil
	}

	if len(clusterUpgrades) > 1 {
		return nil, errors.New("found more than one cluster upgrade")
	}
	return clusterUpgradeFromSQL(clusterUpgrades[0]), nil
}

func (r *repo) ClusterUpgradeGetByVersion(ctx context.Context, tenantID, envID uuid.UUID, version string) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrade, err := r.querier.ClusterUpgradesGetByVersion(ctx, gensql.ClusterUpgradesGetByVersionParams{
		Tenantid: tenantID,
		Envid:    envID,
		Version:  version,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return clusterUpgradeFromSQL(clusterUpgrade), nil
}

func (r *repo) CreateClusterUpgrade(ctx context.Context, tenantID, envID uuid.UUID, version string, isAutomatic *bool) (*model.ClusterUpgradeStatus, error) {
	var isauto pgtype.Bool
	if isAutomatic != nil {
		isauto = pgtype.Bool{Bool: *isAutomatic, Valid: true}
	}

	clusterUpgrade, err := r.querier.ClusterUpgradesCreate(ctx, gensql.ClusterUpgradesCreateParams{
		Tenantid:    tenantID,
		Envid:       envID,
		Version:     version,
		Isautomatic: isauto,
	})
	if err != nil {
		return nil, err
	}

	if isAutomatic != nil && !*isAutomatic {
		r.createAudit(ctx, "manual cluster upgrade to "+version, "cluster_upgrades", clusterUpgrade.ID.String())
	}

	return clusterUpgradeFromSQL(clusterUpgrade), nil
}

// GetActiveClusterOperation returns an active (RUNNING or PENDING) cluster operation for the given tenant and environment, prioritizing RUNNING over PENDING status.
// It checks RUNNING first, then PENDING if no RUNNING operation is found.
func (r *repo) GetActiveClusterOperation(ctx context.Context, tenantID, envID uuid.UUID) (*model.EnvironmentOperation, error) {
	// First try to find a RUNNING operation
	op, err := r.querier.ClusterOperationGet(ctx, gensql.ClusterOperationGetParams{
		Tenantid: tenantID,
		Envid:    envID,
		Status:   "RUNNING",
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// If found, return it
	if err == nil {
		return clusterOperationFromSQL(op), nil
	}

	// No RUNNING operation found, try PENDING
	op, err = r.querier.ClusterOperationGet(ctx, gensql.ClusterOperationGetParams{
		Tenantid: tenantID,
		Envid:    envID,
		Status:   "PENDING",
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	return clusterOperationFromSQL(op), nil
}

func (r *repo) CreateOrUpdateClusterOperation(ctx context.Context, tenantID, envID, upgradeID uuid.UUID, op *containerpb.Operation) (*model.EnvironmentOperation, error) {
	var nodesTotal, nodesFailed, nodesComplete, nodesDone, nodePdbDelaySeconds int

	for _, metric := range op.Progress.GetMetrics() {
		switch metric.Name {
		case "NODES_TOTAL":
			nodesTotal = int(metric.GetIntValue())
		case "NODES_FAILED":
			nodesFailed = int(metric.GetIntValue())
		case "NODES_COMPLETE":
			nodesComplete = int(metric.GetIntValue())
		case "NODES_DONE":
			nodesDone = int(metric.GetIntValue())
		case "NODE_PDB_DELAY_SECONDS":
			nodePdbDelaySeconds = int(metric.GetIntValue())
		}
	}

	var id uuid.UUID
	var err error
	if op.Name != "" {
		uu := strings.SplitAfterN(op.Name, "-", 3)[2]
		id, err = uuid.Parse(uu)
		if err != nil {
			return nil, err
		}
	}

	nodesTotal32, err := ToInt32(nodesTotal)
	if err != nil {
		return nil, err
	}
	nodesFailed32, err := ToInt32(nodesFailed)
	if err != nil {
		return nil, err
	}
	nodesComplete32, err := ToInt32(nodesComplete)
	if err != nil {
		return nil, err
	}
	nodesDone32, err := ToInt32(nodesDone)
	if err != nil {
		return nil, err
	}
	nodePdbDelaySeconds32, err := ToInt32(nodePdbDelaySeconds)
	if err != nil {
		return nil, err
	}

	co, err := r.querier.ClusterOperationCreateOrUpdate(ctx, gensql.ClusterOperationCreateOrUpdateParams{
		ID:                  id,
		OperationName:       op.Name,
		Status:              op.Status.String(),
		TenantID:            tenantID,
		EnvID:               envID,
		UpgradeID:           upgradeID,
		Type:                op.OperationType.String(),
		Target:              op.TargetLink,
		Detail:              op.Detail,
		NodesTotal:          nodesTotal32,
		NodesFailed:         nodesFailed32,
		NodesCompleted:      nodesComplete32,
		NodesDone:           nodesDone32,
		NodePdbDelaySeconds: nodePdbDelaySeconds32,
	})
	if err != nil {
		return &model.EnvironmentOperation{}, err
	}
	return clusterOperationFromSQL(co), nil
}
