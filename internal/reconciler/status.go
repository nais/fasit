package reconciler

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

// FeatureReconcileStatus is the effective reconcile status of a feature
// assignment in a single environment, derived from the reconciler's decision
// and deploy logs plus disabled-feature membership.
type FeatureReconcileStatus struct {
	State        FeatureReconcileStatusState `json:"state"`
	Message      string                      `json:"message"`
	LastModified time.Time                   `json:"lastModified"`
	Created      time.Time                   `json:"created"`

	FeatureAssignmentID uuid.UUID `json:"-"`
	EnvironmentID       uuid.UUID `json:"-"`
}

type FeatureReconcileStatusState string

// ErrNoReconciler is returned when the reconcile-status read path is used
// without a reconciler in the context. The reconciler singleton is placed in
// the context for every HTTP request (see WithContext / run.go).
var ErrNoReconciler = errors.New("reconciler not present in context")

// NormalizeStatus uppercases a status and maps the empty status to UNKNOWN.
func NormalizeStatus(s string) string {
	if s == "" {
		return "UNKNOWN"
	}
	return strings.ToUpper(s)
}

// deriveState collapses the three raw signals into a single display status. The
// disabled-feature membership takes precedence, then the deploy rollout state,
// then the latest reconciler decision interpreted via the Action vocabulary.
func deriveState(deployStatus string, action Action, disabled bool) string {
	switch {
	case disabled:
		return "DISABLED"
	case deployStatus != "":
		return deployStatus
	}
	switch action {
	case ActionSkipDisabled:
		return "DISABLED"
	case ActionFailMissingDeps, ActionFailMissingConfig, ActionFailRender:
		return "failed"
	case ActionSkipUnhealthy, ActionSkipInProgress, ActionDeploy:
		return "pending"
	case ActionSkipUnchanged:
		return "deployed"
	default:
		return "unknown"
	}
}

// ReconcileStatuses returns the effective reconcile status per environment for a
// feature assignment. It reads three simple per-table signals (latest decision,
// deploy rollout state, disabled-feature membership) and joins them by
// environment in Go.
func ReconcileStatuses(ctx context.Context, featureAssignmentID uuid.UUID) ([]*FeatureReconcileStatus, error) {
	r := FromContext(ctx)
	if r == nil {
		return nil, ErrNoReconciler
	}
	return r.reconcileStatuses(ctx, featureAssignmentID)
}

// FeatureStatusForEnvironment returns the reconcile status of the winning
// feature assignment for a feature in an environment. Returns an empty status
// when none exists.
func FeatureStatusForEnvironment(ctx context.Context, envID uuid.UUID, featureName string) (status string, message string, err error) {
	dep, err := featureassignment.WinningAssignment(ctx, envID, featureName)
	if err != nil {
		return "", "", err
	}
	statuses, err := ReconcileStatuses(ctx, dep.ID)
	if err != nil {
		return "", "", err
	}
	for _, s := range statuses {
		if s.EnvironmentID == envID {
			return string(s.State), s.Message, nil
		}
	}
	return "", "", nil
}

func (r *Reconciler) reconcileStatuses(ctx context.Context, featureAssignmentID uuid.UUID) ([]*FeatureReconcileStatus, error) {
	decisions, err := r.querier.ListDecisionStatuses(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list decision statuses: %w", err)
	}
	deploys, err := r.querier.ListDeployStatuses(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list deploy statuses: %w", err)
	}
	disabled, err := r.querier.ListDisabledEnvironments(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list disabled environments: %w", err)
	}

	decByEnv := make(map[uuid.UUID]reconcilersql.ListDecisionStatusesRow, len(decisions))
	for _, d := range decisions {
		decByEnv[d.EnvironmentID] = d
	}
	depByEnv := make(map[uuid.UUID]reconcilersql.ListDeployStatusesRow, len(deploys))
	for _, d := range deploys {
		depByEnv[d.EnvironmentID] = d
	}
	disabledAtByEnv := make(map[uuid.UUID]time.Time, len(disabled))
	for _, d := range disabled {
		disabledAtByEnv[d.EnvironmentID] = d.DisabledAt
	}

	envIDs := make([]uuid.UUID, 0, len(decByEnv))
	seen := make(map[uuid.UUID]struct{})
	addEnv := func(id uuid.UUID) {
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		envIDs = append(envIDs, id)
	}
	for id := range decByEnv {
		addEnv(id)
	}
	for id := range depByEnv {
		addEnv(id)
	}
	for id := range disabledAtByEnv {
		addEnv(id)
	}
	sort.Slice(envIDs, func(i, j int) bool { return envIDs[i].String() < envIDs[j].String() })

	statuses := make([]*FeatureReconcileStatus, len(envIDs))
	for i, envID := range envIDs {
		dec := decByEnv[envID]
		dep := depByEnv[envID]
		disabledAt, isDisabled := disabledAtByEnv[envID]

		state := deriveState(dep.Status, Action(dec.Action), isDisabled)
		lastModified := latestTime(dec.Created, dep.Created, disabledAt)
		statuses[i] = &FeatureReconcileStatus{
			State:               FeatureReconcileStatusState(NormalizeStatus(state)),
			Message:             dec.Message,
			LastModified:        lastModified,
			Created:             lastModified,
			FeatureAssignmentID: featureAssignmentID,
			EnvironmentID:       envID,
		}
	}

	return statuses, nil
}

func latestTime(ts ...time.Time) time.Time {
	var latest time.Time
	for _, t := range ts {
		if t.After(latest) {
			latest = t
		}
	}
	return latest
}
