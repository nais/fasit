package featureassignment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	commonmodel "github.com/nais/fasit/internal/model"
)

var ErrFeatureNotFound = fmt.Errorf("feature not found")

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

	id, err := createFeatureAssignment(ctx, feat, in.Description, in.Commit, in.Target)
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

func ListForEnvironment(ctx context.Context, envID uuid.UUID) ([]*FeatureAssignment, error) {
	rows, err := querier(ctx).ListFeatureAssignmentsForEnvironment(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("list feature assignments for environment: %w", err)
	}
	deps, err := featureAssignmentsFromRows(rows)
	if err != nil {
		return nil, err
	}
	return mostSpecificPerFeature(deps), nil
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
			LastModified:        status.LastModified,
			Created:             status.Created,
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

func Deactivate(ctx context.Context, featureAssignmentID uuid.UUID) (string, error) {
	var featureName string
	err := dbtx.WithTx(ctx, func(ctx context.Context) error {
		objectID := featureAssignmentID.String()
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
	return featureName, err
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
		Action:        audit.ActionRedeploy,
		ObjectType:    audit.ObjectTypeFeatureAssignment,
		ObjectID:      featureName,
		Feature:       featureName,
		EnvironmentID: &envID,
	})

	return nil
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

func HasActiveAssignments(ctx context.Context, featureName string) (bool, error) {
	return querier(ctx).HasActiveAssignments(ctx, featureName)
}

// IsMoreSpecific reports whether a deployment with candidateLabels (created at
// candidateCreated) should replace one with existingLabels (created at
// existingCreated). More target labels means more specific. Equal count: latest wins.
func IsMoreSpecific(candidateLabels, existingLabels map[string]string, candidateCreated, existingCreated time.Time) bool {
	if len(candidateLabels) > len(existingLabels) {
		return true
	}
	return len(candidateLabels) == len(existingLabels) && candidateCreated.After(existingCreated)
}

// mostSpecificPerFeature picks one feature assignment per feature name: the one with
// the most specific target labels (most labels wins), breaking ties by latest
// created timestamp.
func mostSpecificPerFeature(deps []*FeatureAssignment) []*FeatureAssignment {
	assignments := map[string]*FeatureAssignment{}
	for _, dep := range deps {
		existing, ok := assignments[dep.Feature.Name]
		if !ok || IsMoreSpecific(dep.TargetLabels, existing.TargetLabels, dep.Created, existing.Created) {
			assignments[dep.Feature.Name] = dep
		}
	}

	ret := make([]*FeatureAssignment, 0)
	for _, d := range assignments {
		ret = append(ret, d)
	}

	slices.SortStableFunc(ret, func(a, b *FeatureAssignment) int {
		return a.Created.Compare(b.Created)
	})

	return ret
}

func createFeatureAssignment(ctx context.Context, feat *model.Feature, description *string, githubRef *commonmodel.GitHubCommit, target environment.Labels) (uuid.UUID, error) {
	details, err := featurepkg.ParseTemplateDetails(feat.Values)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to parse feature template details: %w", err)
	}

	if err := featurepkg.FeatureDataCreate(ctx, *feat, details); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return uuid.Nil, fmt.Errorf("unable to create feature data: %w", pgErr)
		}
	}

	var ghRef []byte
	if githubRef != nil {
		b, err := json.Marshal(githubRef.Ref)
		if err != nil {
			return uuid.Nil, fmt.Errorf("marshal gh ref: %w", err)
		}

		ghRef = b
	}

	var assignment featureassignmentsql.FeatureAssignment
	err = dbtx.WithTx(ctx, func(ctx context.Context) error {
		err = querier(ctx).DeactivateActiveFeatureAssignmentForTarget(ctx, featureassignmentsql.DeactivateActiveFeatureAssignmentForTargetParams{
			FeatureName: feat.Name,
			Target:      types.EnvironmentLabels(target),
		})
		if err != nil {
			return fmt.Errorf("deactivate previous assignment: %w", err)
		}

		assignment, err = querier(ctx).CreateFeatureAssignment(ctx, featureassignmentsql.CreateFeatureAssignmentParams{
			FeatureName: feat.Name,
			Version:     feat.Version,
			GhRef:       ghRef,
			Target:      types.EnvironmentLabels(target),
			Description: description,
		})
		if err != nil {
			return fmt.Errorf("unable to create feature assignment: %w", err)
		}

		return nil
	})
	if err != nil {
		return uuid.Nil, err
	}

	return assignment.ID, nil
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
