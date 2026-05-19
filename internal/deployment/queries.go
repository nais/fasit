package deployment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/sirupsen/logrus"
)

type ctxKey int

const managerKey ctxKey = iota

func Register(ctx context.Context, deploymentManager *Manager) context.Context {
	return context.WithValue(ctx, managerKey, deploymentManager)
}

func RegisterForTest(ctx context.Context, querier deploymentsql.Querier) context.Context {
	return context.WithValue(ctx, managerKey, &Manager{querier: querier})
}

func fromContext(ctx context.Context) *Manager {
	return ctx.Value(managerKey).(*Manager)
}

func querier(ctx context.Context) deploymentsql.Querier {
	q := fromContext(ctx).querier
	if tx, ok := dbtx.Tx(ctx); ok {
		if real, ok := q.(*deploymentsql.Queries); ok {
			return real.WithTx(tx)
		}
	}
	return q
}

func GetManager(ctx context.Context) *Manager {
	return fromContext(ctx)
}

func deploymentsFromRows(rows []deploymentsql.ListDeploymentsForEnvironmentRow) ([]*Deployment, error) {
	deps := make([]*Deployment, len(rows))
	for i, row := range rows {
		dep, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		dep.TplDetails = row.FeatureDatum.TplDetails
		dep.Disabled = row.Disabled
		deps[i] = dep
	}
	return deps, nil
}

func ValueRefsForEnvironment(ctx context.Context, envID uuid.UUID) (map[string][]string, error) {
	rows, err := querier(ctx).ListDeploymentsForEnvironment(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("list deployments for environment: %w", err)
	}
	deps, err := deploymentsFromRows(rows)
	if err != nil {
		return nil, err
	}
	return collectKeyRefs(filterDeployments(deps)), nil
}

// TriggerReconcile will trigger an asynchronous reconciliation of deployments. The returned channel can be used to wait
// for the result.
func TriggerReconcile(ctx context.Context, event ReconcileTriggerEvent) chan TriggerResult {
	return fromContext(ctx).reconciler.trigger(event)
}

func CreateDeployment(ctx context.Context, req Request) (uuid.UUID, error) {
	feat, err := ChartDownloader(req.Chart, req.Version)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to convert oci chart: %w", err)
	}

	if len(feat.EnvironmentKinds) == 0 {
		return uuid.Nil, fmt.Errorf("no environments defined in Feature.yaml")
	}

	if feat.Source == "" {
		return uuid.Nil, fmt.Errorf("no source url found in Chart.yaml")
	}

	id, err := fromContext(ctx).deployer.CreateDeployment(ctx, feat, req)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func GetDeployment(ctx context.Context, id uuid.UUID) (*Deployment, error) {
	return getDeployment(ctx, querier(ctx), id)
}

func GetDeployInstructionByID(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error) {
	di, err := querier(ctx).DeployInstructionsByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return deployInstructionFromSQL(di), nil
}

func ListDeployInstructionsByDeploymentID(ctx context.Context, deploymentID uuid.UUID) ([]deploymentsql.ListDeployInstructionsByDeploymentIDRow, error) {
	return querier(ctx).ListDeployInstructionsByDeploymentID(ctx, &deploymentID)
}

func GetDeploymentStatusLog(ctx context.Context, deploymentID, environmentID uuid.UUID) (*model.RolloutLog, error) {
	di, err := querier(ctx).GetDeployInstructionByDeploymentAndEnvironmentID(ctx, deploymentsql.GetDeployInstructionByDeploymentAndEnvironmentIDParams{
		DeploymentID:  &deploymentID,
		EnvironmentID: environmentID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get deploy instruction: %w", err)
	}

	lines, err := featurepkg.LogsGet(ctx, di.DeployInstruction.ID)
	if err != nil {
		return nil, fmt.Errorf("get logs: %w", err)
	}

	return &model.RolloutLog{
		ID:          di.DeployInstruction.ID,
		TenantName:  di.TenantName,
		Environment: di.EnvironmentName,
		Lines:       lines,
	}, nil
}

func ListDeployments(ctx context.Context) ([]*Deployment, error) {
	rows, err := querier(ctx).ListDeployments(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*Deployment, len(rows))
	for i, row := range rows {
		deployment, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		ret[i] = deployment
	}

	return ret, nil
}

func ListDeploymentStatuses(ctx context.Context, deploymentID uuid.UUID) ([]*DeploymentStatus, error) {
	rows, err := querier(ctx).ListDeploymentStatuses(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("get deployment statuses: %w", err)
	}

	models := make([]*DeploymentStatus, len(rows))
	for i, status := range rows {
		models[i] = deploymentStatusFromSQL(deploymentsql.DeploymentStatus(status))
	}

	return models, nil
}

func ListDeploymentsByFeature(ctx context.Context, featureName string) ([]*Deployment, error) {
	rows, err := querier(ctx).ListDeploymentsByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	ret := make([]*Deployment, len(rows))
	for i, row := range rows {
		deployment, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		ret[i] = deployment
	}

	return ret, nil
}

func DeleteDeployment(ctx context.Context, deploymentID uuid.UUID) error {
	err := querier(ctx).DeleteDeployment(ctx, deploymentID)
	if err != nil {
		return err
	}
	audit.CreateAudit(ctx, "deleted", "deployments", deploymentID.String())
	return nil
}

func DeleteDeploymentsByFeatureAndTarget(ctx context.Context, featureName string, target types.EnvironmentLabels) error {
	err := querier(ctx).DeleteDeploymentsByFeatureAndTarget(ctx, deploymentsql.DeleteDeploymentsByFeatureAndTargetParams{
		FeatureName: featureName,
		Target:      target,
	})
	if err != nil {
		return err
	}
	audit.CreateAudit(ctx, "deleted all deployments matching feature and target", "deployments", featureName)
	return nil
}

func TriggerRedeploy(ctx context.Context, envID uuid.UUID, featureName string) error {
	if err := invalidateDeployInstructionHash(ctx, envID, featureName); err != nil {
		return fmt.Errorf("invalidate hash: %w", err)
	}
	TriggerReconcile(ctx, ReconcileTriggerEvent{})
	return nil
}

func invalidateDeployInstructionHash(ctx context.Context, envID uuid.UUID, featureName string) error {
	return querier(ctx).InvalidateDeployInstructionHash(ctx, deploymentsql.InvalidateDeployInstructionHashParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
}

type EnvironmentFeature struct {
	Name     string
	Disabled bool
}

func ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]EnvironmentFeature, error) {
	rows, err := querier(ctx).ListDeploymentsForEnvironment(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("list deployments for environment: %w", err)
	}
	deps, err := deploymentsFromRows(rows)
	if err != nil {
		return nil, err
	}
	filtered := filterDeployments(deps)
	seen := make(map[string]bool, len(filtered))
	features := make([]EnvironmentFeature, 0, len(filtered))
	for _, dep := range filtered {
		if !seen[dep.Feature.Name] {
			seen[dep.Feature.Name] = true
			features = append(features, EnvironmentFeature{
				Name:     dep.Feature.Name,
				Disabled: dep.Disabled,
			})
		}
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Name < features[j].Name })
	return features, nil
}

func FeatureForEnvironment(ctx context.Context, envID uuid.UUID, featureName string) (*model.Feature, error) {
	rows, err := querier(ctx).ListDeploymentsForEnvironmentFeature(ctx, deploymentsql.ListDeploymentsForEnvironmentFeatureParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, fmt.Errorf("query deployments for feature %q: %w", featureName, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("no deployment found for feature %q in environment", featureName)
	}
	deps := make([]*Deployment, len(rows))
	for i, row := range rows {
		dep, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		deps[i] = dep
	}
	winner := filterDeployments(deps)
	if len(winner) == 0 {
		return nil, fmt.Errorf("no deployment found for feature %q in environment after filtering", featureName)
	}
	return winner[0].Feature, nil
}

// TimeoutDeployInstructions will periodically check for deploy instructions that have been in pending state for
// more than one hour and mark them as failed
func TimeoutDeployInstructions(ctx context.Context, log logrus.FieldLogger) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		err := querier(ctx).TimeoutDeployInstructions(ctx)
		if err != nil {
			log.WithError(err).Error("failed to timeout deploy instructions")
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func SetReleaseStatus(ctx context.Context, environmentID uuid.UUID, h *message.Release) error {
	_, err := querier(ctx).SetReleaseStatus(ctx, deploymentsql.SetReleaseStatusParams{
		EnvironmentID: environmentID,
		Feature:       h.Name,
		Version:       h.Version,
		Status:        h.Status,
		Revision:      int32(h.Revision), // #nosec G115
		LastDeployed: pgtype.Timestamptz{
			Time:  h.LastDeployed,
			Valid: true,
		},
	})

	return err
}

func ListReleaseStatuses(ctx context.Context, environmentID uuid.UUID) ([]*model.Release, error) {
	res, err := querier(ctx).ListReleaseStatuses(ctx, environmentID)
	if err != nil {
		return nil, err
	}
	releases := make([]*model.Release, len(res))
	for i, r := range res {
		releases[i] = &model.Release{
			Name:         r.Feature,
			Version:      r.Version,
			Status:       r.Status,
			Revision:     int(r.Revision),
			LastDeployed: r.LastDeployed.Time,
			Created:      r.Created.Time,
			LastModified: r.LastModified.Time,
		}
	}

	return releases, nil
}

func DeleteReleaseStatus(ctx context.Context, environmentID uuid.UUID) error {
	return querier(ctx).ReleaseStatusDeleteByEnvironment(ctx, environmentID)
}

func UpdateDeployInstructionStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %q", status)
	}
	return querier(ctx).UpdateDeployInstructionStatus(ctx, deploymentsql.UpdateDeployInstructionStatusParams{
		ID:     id,
		Status: status.String(),
	})
}
