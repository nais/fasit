package database

import (
	"context"
	"errors"
	"math"
	"strings"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

// toInt32 safely converts int to int32, returning an error if out of bounds
func toInt32(val int) (int32, error) {
	if val > math.MaxInt32 || val < math.MinInt32 {
		return 0, errors.New("toInt32: value out of int32 bounds")
	}
	return int32(val), nil
}

type ClusterUpgraderRepo interface {
	CreateOrUpdateClusterOperation(ctx context.Context, tenantID, envID, versionID uuid.UUID, op *containerpb.Operation) (*model.EnvironmentOperation, error)
	GetRunningClusterOperation(ctx context.Context, tenantID, envID uuid.UUID) (*model.EnvironmentOperation, error)
	CreateClusterUpgrade(ctx context.Context, tenantID, envID uuid.UUID, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGet(ctx context.Context, tenantID, envID uuid.UUID) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeHistoryGet(ctx context.Context, tenantID, envID uuid.UUID) ([]*model.ClusterUpgradeStatus, error)
	UpdateClusterUpgradeStatus(ctx context.Context, tenantID, envID uuid.UUID, status gensql.ClusterUpgradesStatus, version string) (*model.ClusterUpgradeStatus, error)
	ClusterUpgradeGetByID(ctx context.Context, id uuid.UUID) (*model.ClusterUpgradeStatus, error)
	ClusterOperationsGetByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentOperation, error)
	ClusterOperationsGetByUpgradeID(ctx context.Context, upgradeID uuid.UUID) ([]*model.EnvironmentOperation, error)
	SetClusterUpgradesSlackMessage(ctx context.Context, id uuid.UUID, slackMessageTS, channelID string) (*model.ClusterUpgradeStatus, error)
}

func (r *repo) ClusterUpgradeHistoryGet(ctx context.Context, tenantID, envID uuid.UUID) ([]*model.ClusterUpgradeStatus, error) {
	clusterUpgrades, err := r.querier.ClusterUpgradesHistoryGetByEnvironmentID(ctx, gensql.ClusterUpgradesHistoryGetByEnvironmentIDParams{
		Tenantid: tenantID,
		Envid:    envID,
	})
	if err != nil {
		return nil, err
	}

	var upgrades []*model.ClusterUpgradeStatus
	for _, upgrade := range clusterUpgrades {
		upgrades = append(upgrades, clusterUpgradeFromSQL(upgrade))
	}

	return upgrades, nil
}

func clusterUpgradeFromSQL(p gensql.ClusterUpgrade) *model.ClusterUpgradeStatus {
	return &model.ClusterUpgradeStatus{
		ID:                    p.ID,
		Version:               p.Version,
		UpgradeStatus:         model.UpgradeStatus(p.Status),
		LastModified:          p.LastModified.Time,
		StartTime:             p.StartTime.Time,
		EnvironmentID:         p.EnvironmentID,
		SlackMessageTimestamp: p.SlackMessageTimestamp.String,
		SlackChannelID:        p.SlackChannelID.String,
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

func (r *repo) UpdateClusterUpgradeStatus(ctx context.Context, tenantID, envID uuid.UUID, status gensql.ClusterUpgradesStatus, version string) (*model.ClusterUpgradeStatus, error) {
	clusterUpgrade, err := r.querier.ClusterUpgradesUpdateStatus(ctx, gensql.ClusterUpgradesUpdateStatusParams{
		Status:   status,
		Tenantid: tenantID,
		Envid:    envID,
		Version:  version,
	})
	if err != nil {
		return nil, err
	}

	return clusterUpgradeFromSQL(clusterUpgrade), nil
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

func (r *repo) CreateClusterUpgrade(ctx context.Context, tenantID, envID uuid.UUID, version string) (*model.ClusterUpgradeStatus, error) {
	clusterVersion, err := r.querier.ClusterUpgradesCreate(ctx, gensql.ClusterUpgradesCreateParams{
		Tenantid: tenantID,
		Envid:    envID,
		Version:  version,
	})
	if err != nil {
		return nil, err
	}

	return clusterUpgradeFromSQL(clusterVersion), nil
}

func (r *repo) GetRunningClusterOperation(ctx context.Context, tenantID, envID uuid.UUID) (*model.EnvironmentOperation, error) {
	op, err := r.querier.ClusterOperationGet(ctx, gensql.ClusterOperationGetParams{
		Tenantid: tenantID,
		Envid:    envID,
		Status:   "RUNNING",
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
		NodesTotal:          toInt32(nodesTotal),
		NodesFailed:         toInt32(nodesFailed),
		NodesCompleted:      toInt32(nodesComplete),
		NodesDone:           toInt32(nodesDone),
		NodePdbDelaySeconds: toInt32(nodePdbDelaySeconds),
	})
	if err != nil {
		return &model.EnvironmentOperation{}, err
	}
	return clusterOperationFromSQL(co), nil
}
