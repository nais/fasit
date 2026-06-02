package featureassignment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
)

var ErrFeatureNotFound = fmt.Errorf("feature not found")

func ValueRefsForEnvironment(ctx context.Context, envID uuid.UUID) (map[string][]string, error) {
	rows, err := querier(ctx).ListFeatureAssignmentsForEnvironment(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("list feature assignments for environment: %w", err)
	}
	deps, err := featureAssignmentsFromRows(rows)
	if err != nil {
		return nil, err
	}
	return collectKeyRefs(mostSpecificPerFeature(deps)), nil
}

// TriggerReconcile will trigger an asynchronous reconciliation of deployments.
// func TriggerReconcile(ctx context.Context) {
// 	r := fromContext(ctx).reconciler
//
// 	select {
// 	case r.reconcileTrigger <- struct{}{}:
// 	default:
// 		r.log.Debug("there is already a reconcile event queued, skipping")
// 	}
// }

func RunReconciler(ctx context.Context, interval time.Duration) {
	fromContext(ctx).reconciler.Run(ctx, interval)
}

func Create(ctx context.Context, in CreateFeatureAssignment) (uuid.UUID, error) {
	feat, err := ChartDownloader(in.Chart, in.Version)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to convert oci chart: %w", err)
	}

	if len(feat.EnvironmentKinds) == 0 {
		return uuid.Nil, fmt.Errorf("no environments defined in Feature.yaml")
	}

	if feat.Source == "" {
		return uuid.Nil, fmt.Errorf("no source url found in Chart.yaml")
	}

	id, err := fromContext(ctx).deployer.CreateFeatureAssignment(ctx, feat, in.Description, in.Commit, in.Target)
	if err != nil {
		return uuid.Nil, err
	}

	_ = audit.Create(ctx, audit.CreateParams{
		Action:      audit.ActionCreated,
		Description: "version " + in.Version + " \u2192 " + formatLabels(in.Target),
		ObjectType:  audit.ObjectTypeFeatureAssignment,
		ObjectID:    feat.Name,
		Feature:     feat.Name,
		Metadata: map[string]any{
			"deploymentId": id.String(),
			"chart":        in.Chart,
			"target":       in.Target,
		},
	})

	return id, nil
}

func Get(ctx context.Context, featureAssignmentID uuid.UUID) (*FeatureAssignment, error) {
	d, err := querier(ctx).GetFeatureAssignment(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("getting feature assignment from db: %w", err)
	}

	ret, err := featureAssignmentFromSQL(d.FeatureAssignment, d.FeatureDatum)
	if err != nil {
		return nil, fmt.Errorf("converting feature assignment from sql: %w", err)
	}

	return ret, nil
}

func GetDeployInstruction(ctx context.Context, deployInstructionID uuid.UUID) (*model.DeployInstruction, error) {
	di, err := querier(ctx).GetDeployInstruction(ctx, deployInstructionID)
	if err != nil {
		return nil, err
	}
	return &model.DeployInstruction{
		ID:                  di.ID,
		EnvironmentID:       di.EnvironmentID,
		FeatureAssignmentID: di.FeatureAssignmentID,
		FeatureName:         di.FeatureName,
		FeatureVersion:      di.FeatureVersion,
		Status:              model.RolloutStatus(di.Status),
		Hash:                di.Hash,
		Created:             di.Created.Time,
		LastModified:        di.LastModified.Time,
		Values:              di.Values,
	}, nil
}

func ListDeployInstructions(ctx context.Context, featureAssignmentID uuid.UUID) ([]featureassignmentsql.ListDeployInstructionsRow, error) {
	return querier(ctx).ListDeployInstructions(ctx, &featureAssignmentID)
}

func GetFeatureReconcileStatusLog(ctx context.Context, featureAssignmentID, environmentID uuid.UUID) (*model.RolloutLog, error) {
	di, err := querier(ctx).GetDeployInstructionByFeatureAssignmentAndEnvironmentID(ctx, featureassignmentsql.GetDeployInstructionByFeatureAssignmentAndEnvironmentIDParams{
		FeatureAssignmentID: &featureAssignmentID,
		EnvironmentID:       environmentID,
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

func ListAll(ctx context.Context) ([]*FeatureAssignment, error) {
	rows, err := querier(ctx).ListAllFeatureAssignments(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*FeatureAssignment, len(rows))
	for i, row := range rows {
		assignment, err := featureAssignmentFromSQL(row.FeatureAssignment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature assignment: %w", err)
		}
		ret[i] = assignment
	}

	return ret, nil
}

func ListRecent(ctx context.Context) ([]*FeatureAssignment, error) {
	rows, err := querier(ctx).ListRecentFeatureAssignments(ctx)
	if err != nil {
		return nil, err
	}

	ret := make([]*FeatureAssignment, len(rows))
	for i, row := range rows {
		assignment, err := featureAssignmentFromSQL(row.FeatureAssignment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature assignment: %w", err)
		}
		ret[i] = assignment
	}

	return ret, nil
}

func ListFeatureReconcileStatuses(ctx context.Context, featureAssignmentID uuid.UUID) ([]*FeatureReconcileStatus, error) {
	rows, err := querier(ctx).ListReconcileStatuses(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("get feature assignment statuses: %w", err)
	}

	models := make([]*FeatureReconcileStatus, len(rows))
	for i, status := range rows {
		models[i] = &FeatureReconcileStatus{
			State:               FeatureReconcileStatusState(strings.ToUpper(status.Status)),
			Message:             status.Message,
			LastModified:        status.LastModified.Time,
			Created:             status.Created.Time,
			FeatureAssignmentID: status.FeatureAssignmentID,
			EnvironmentID:       status.EnvironmentID,
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

// FeatureStatusForEnvironment returns the feature assignment status for the winning
// feature assignment of a feature in an environment. Returns empty string when no status exists.
func FeatureStatusForEnvironment(ctx context.Context, envID uuid.UUID, featureName string) (status string, message string, err error) {
	dep, err := mostSpecificAssignment(ctx, envID, featureName)
	if err != nil {
		return "", "", err
	}
	row, err := querier(ctx).GetReconcileStatus(ctx, featureassignmentsql.GetReconcileStatusParams{
		FeatureAssignmentID: dep.ID,
		EnvironmentID:       envID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("get reconcile status: %w", err)
	}
	return NormalizeStatus(row.Status), row.Message, nil
}

func ListByFeature(ctx context.Context, featureName string) ([]*FeatureAssignment, error) {
	rows, err := querier(ctx).ListFeatureAssignmentsByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	ret := make([]*FeatureAssignment, len(rows))
	for i, row := range rows {
		assignment, err := featureAssignmentFromSQL(row.FeatureAssignment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature assignment: %w", err)
		}
		ret[i] = assignment
	}

	return ret, nil
}

func ListAllByFeature(ctx context.Context, featureName string) ([]*FeatureAssignment, error) {
	rows, err := querier(ctx).ListAllFeatureAssignmentsByFeature(ctx, featureName)
	if err != nil {
		return nil, err
	}

	ret := make([]*FeatureAssignment, len(rows))
	for i, row := range rows {
		assignment, err := featureAssignmentFromSQL(row.FeatureAssignment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature assignment: %w", err)
		}
		ret[i] = assignment
	}

	return ret, nil
}

func Deactivate(ctx context.Context, featureAssignmentID uuid.UUID) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		objectID := featureAssignmentID.String()
		featureName := ""
		if row, err := querier(ctx).GetFeatureAssignment(ctx, featureAssignmentID); err == nil {
			objectID = row.FeatureAssignment.FeatureName
			featureName = row.FeatureAssignment.FeatureName
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}

		err := querier(ctx).DeactivateFeatureAssignment(ctx, featureAssignmentID)
		if err != nil {
			return err
		}
		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionDeleted,
			ObjectType: audit.ObjectTypeFeatureAssignment,
			ObjectID:   objectID,
			Feature:    featureName,
			Metadata: map[string]any{
				"deploymentID": featureAssignmentID.String(),
			},
		})
	})
}

func DeactivateByFeatureAndTarget(ctx context.Context, featureName string, target types.EnvironmentLabels) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err := querier(ctx).DeactivateFeatureAssignmentsByFeatureAndTarget(ctx, featureassignmentsql.DeactivateFeatureAssignmentsByFeatureAndTargetParams{
			FeatureName: featureName,
			Target:      target,
		})
		if err != nil {
			return err
		}
		return audit.Create(ctx, audit.CreateParams{
			Action:     audit.ActionDeleted,
			ObjectType: audit.ObjectTypeFeatureAssignment,
			ObjectID:   featureName,
			Metadata: map[string]any{
				"target": target,
			},
		})
	})
}

func InvalidateLatestDeploy(ctx context.Context, envID uuid.UUID, featureName string) error {
	err := querier(ctx).InvalidateDeployInstruction(ctx, featureassignmentsql.InvalidateDeployInstructionParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return fmt.Errorf("invalidate hash: %w", err)
	}

	if dep, err := mostSpecificAssignment(ctx, envID, featureName); err == nil {
		_ = querier(ctx).SetReconcileStatus(ctx, featureassignmentsql.SetReconcileStatusParams{
			FeatureAssignmentID: dep.ID,
			EnvironmentID:       envID,
			Status:              "pending",
			Message:             "redeploy triggered",
		})
	}

	_ = audit.Create(ctx, audit.CreateParams{
		Action:        audit.ActionTriggered,
		ObjectType:    audit.ObjectTypeFeatureAssignment,
		ObjectID:      featureName,
		Feature:       featureName,
		EnvironmentID: &envID,
	})

	return nil
}

func ListEnvironmentFeatures(ctx context.Context, environmentID uuid.UUID) ([]EnvironmentFeature, error) {
	rows, err := querier(ctx).ListFeatureAssignmentsForEnvironment(ctx, environmentID)
	if err != nil {
		return nil, fmt.Errorf("list feature assignments for environment: %w", err)
	}
	deps, err := featureAssignmentsFromRows(rows)
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
	dep, err := mostSpecificAssignment(ctx, envID, featureName)
	if err != nil {
		return nil, err
	}
	return dep.Feature, nil
}

// WinningAssignment returns the most specific active feature assignment for a feature
// in a given environment.
func WinningAssignment(ctx context.Context, envID uuid.UUID, featureName string) (*FeatureAssignment, error) {
	return mostSpecificAssignment(ctx, envID, featureName)
}

func mostSpecificAssignment(ctx context.Context, envID uuid.UUID, featureName string) (*FeatureAssignment, error) {
	rows, err := querier(ctx).ListFeatureAssignmentsForFeatureInEnvironment(ctx, featureassignmentsql.ListFeatureAssignmentsForFeatureInEnvironmentParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, fmt.Errorf("query feature assignments for feature %q: %w", featureName, err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %q in environment", ErrFeatureNotFound, featureName)
	}
	deps := make([]*FeatureAssignment, len(rows))
	for i, row := range rows {
		dep, err := featureAssignmentFromSQL(row.FeatureAssignment, row.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature assignment: %w", err)
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
func TimeoutDeployInstructions(ctx context.Context, log *slog.Logger) {
	for {
		err := querier(ctx).TimeoutDeployInstructions(ctx)
		if err != nil {
			log.Error("failed to timeout deploy instructions", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Minute):
		}
	}
}

func SetReleaseStatus(ctx context.Context, environmentID uuid.UUID, h *message.Release) error {
	_, err := querier(ctx).SetReleaseStatus(ctx, featureassignmentsql.SetReleaseStatusParams{
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
	return querier(ctx).UpdateDeployInstructionStatus(ctx, featureassignmentsql.UpdateDeployInstructionStatusParams{
		ID:     id,
		Status: status.String(),
	})
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "all environments"
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+labels[k])
	}
	return strings.Join(parts, ", ")
}
