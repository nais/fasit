package deployment

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
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

var ErrFeatureNotFound = fmt.Errorf("feature not found")

func ValueRefsForEnvironment(ctx context.Context, envID uuid.UUID) (map[string][]string, error) {
	rows, err := querier(ctx).ListDeploymentsForEnvironment(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("list deployments for environment: %w", err)
	}
	deps, err := deploymentsFromRows(rows)
	if err != nil {
		return nil, err
	}
	return collectKeyRefs(mostSpecificPerFeature(deps)), nil
}

// TriggerReconcile will trigger an asynchronous reconciliation of deployments. The returned channel can be used to wait
// for the result.
func TriggerReconcile(ctx context.Context, event ReconcileTriggerEvent) chan TriggerResult {
	return fromContext(ctx).reconciler.trigger(event)
}

func RunReconciler(ctx context.Context, interval time.Duration) {
	fromContext(ctx).reconciler.Run(ctx, interval)
}

func Create(ctx context.Context, req Request) (uuid.UUID, error) {
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

func Get(ctx context.Context, deploymentID uuid.UUID) (*Deployment, error) {
	d, err := querier(ctx).GetDeployment(ctx, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("getting deployment from db: %w", err)
	}

	ret, err := deploymentFromSQL(d.Deployment, d.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("converting deployment from sql: %w", err)
	}

	return ret, nil
}

func GetDeployInstruction(ctx context.Context, deployInstructionID uuid.UUID) (*model.DeployInstruction, error) {
	di, err := querier(ctx).GetDeployInstruction(ctx, deployInstructionID)
	if err != nil {
		return nil, err
	}
	return &model.DeployInstruction{
		ID:             di.ID,
		EnvironmentID:  di.EnvironmentID,
		DeploymentID:   di.DeploymentID,
		FeatureName:    di.FeatureName,
		FeatureVersion: di.FeatureVersion,
		Status:         model.RolloutStatus(di.Status),
		Hash:           di.Hash,
		Created:        di.Created.Time,
		LastModified:   di.LastModified.Time,
		Values:         di.Values,
	}, nil
}

func ListDeployInstructions(ctx context.Context, deploymentID uuid.UUID) ([]deploymentsql.ListDeployInstructionsRow, error) {
	return querier(ctx).ListDeployInstructions(ctx, &deploymentID)
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

func List(ctx context.Context) ([]*Deployment, error) {
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
		models[i] = &DeploymentStatus{
			State:         DeploymentStatusState(strings.ToUpper(status.Status)),
			Message:       status.Message,
			LastModified:  status.LastModified.Time,
			Created:       status.Created.Time,
			DeploymentID:  status.DeploymentID,
			EnvironmentID: status.EnvironmentID,
		}
	}

	return models, nil
}

// NormalizeStatus uppercases a deployment status and maps intermediate
// states to user-facing labels (e.g. CREATED → PENDING).
func NormalizeStatus(s string) string {
	s = strings.ToUpper(s)
	switch s {
	case "CREATED", "INVALIDATED":
		return "PENDING"
	case "":
		return "UNKNOWN"
	default:
		return s
	}
}

// FeatureStatusForEnvironment returns the deployment status for the winning
// deployment of a feature in an environment. Returns empty string when no status exists.
func FeatureStatusForEnvironment(ctx context.Context, envID uuid.UUID, featureName string) (status string, message string, err error) {
	dep, err := mostSpecificDeployment(ctx, envID, featureName)
	if err != nil {
		return "", "", err
	}
	row, err := querier(ctx).GetDeploymentStatus(ctx, deploymentsql.GetDeploymentStatusParams{
		DeploymentID:  dep.ID,
		EnvironmentID: envID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("get deployment status: %w", err)
	}
	return NormalizeStatus(row.Status), row.Message, nil
}

func ListByFeature(ctx context.Context, featureName string) ([]*Deployment, error) {
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

func Delete(ctx context.Context, deploymentID uuid.UUID) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err := querier(ctx).DeleteDeployment(ctx, deploymentID)
		if err != nil {
			return err
		}
		return audit.Create(ctx, audit.CreateParams{
			Description: "deleted",
			ObjectType:  "deployments",
			ObjectID:    deploymentID.String(),
		})
	})
}

func DeleteDeploymentsByFeatureAndTarget(ctx context.Context, featureName string, target types.EnvironmentLabels) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err := querier(ctx).DeleteDeploymentsByFeatureAndTarget(ctx, deploymentsql.DeleteDeploymentsByFeatureAndTargetParams{
			FeatureName: featureName,
			Target:      target,
		})
		if err != nil {
			return err
		}
		return audit.Create(ctx, audit.CreateParams{
			Description: "deleted all deployments matching feature and target",
			ObjectType:  "deployments",
			ObjectID:    featureName,
		})
	})
}

func TriggerRedeploy(ctx context.Context, envID uuid.UUID, featureName string) error {
	err := querier(ctx).InvalidateDeployInstruction(ctx, deploymentsql.InvalidateDeployInstructionParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return fmt.Errorf("invalidate hash: %w", err)
	}

	if dep, err := mostSpecificDeployment(ctx, envID, featureName); err == nil {
		_ = querier(ctx).SetDeploymentStatus(ctx, deploymentsql.SetDeploymentStatusParams{
			DeploymentID:  dep.ID,
			EnvironmentID: envID,
			Status:        "pending",
			Message:       "redeploy triggered",
		})
	}

	TriggerReconcile(ctx, ReconcileTriggerEvent{})
	return nil
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
	filtered := mostSpecificPerFeature(deps)
	seen := make(map[string]bool, len(filtered))
	features := make([]EnvironmentFeature, 0, len(filtered))
	for _, dep := range filtered {
		if !seen[dep.Feature.Name] {
			seen[dep.Feature.Name] = true
			features = append(features, EnvironmentFeature{
				Name:            dep.Feature.Name,
				FeatureDisabled: dep.FeatureDisabled,
			})
		}
	}
	sort.Slice(features, func(i, j int) bool { return features[i].Name < features[j].Name })
	return features, nil
}

func FeatureForEnvironment(ctx context.Context, envID uuid.UUID, featureName string) (*model.Feature, error) {
	dep, err := mostSpecificDeployment(ctx, envID, featureName)
	if err != nil {
		return nil, err
	}
	return dep.Feature, nil
}

func mostSpecificDeployment(ctx context.Context, envID uuid.UUID, featureName string) (*Deployment, error) {
	rows, err := querier(ctx).ListDeploymentsForFeatureInEnvironment(ctx, deploymentsql.ListDeploymentsForFeatureInEnvironmentParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, fmt.Errorf("query deployments for feature %q: %w", featureName, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %q in environment", ErrFeatureNotFound, featureName)
	}
	deps := make([]*Deployment, len(rows))
	for i, row := range rows {
		dep, err := deploymentFromSQL(row.Deployment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make deployment: %w", err)
		}
		deps[i] = dep
	}
	winner := mostSpecificPerFeature(deps)
	if len(winner) == 0 {
		return nil, fmt.Errorf("%w: %q in environment after filtering", ErrFeatureNotFound, featureName)
	}
	return winner[0], nil
}

// TimeoutDeployInstructions will periodically check for deploy instructions that have been in pending state for
// more than one hour and mark them as failed
func TimeoutDeployInstructions(ctx context.Context, log logrus.FieldLogger) {
	for {
		err := querier(ctx).TimeoutDeployInstructions(ctx)
		if err != nil {
			log.WithError(err).Error("failed to timeout deploy instructions")
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Minute):
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
	return querier(ctx).DeleteReleaseStatusesInEnvironment(ctx, environmentID)
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
