package reconciler

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/feature"
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

// NormalizeStatus uppercases a status and maps the empty status to UNKNOWN.
func NormalizeStatus(s string) string {
	if s == "" {
		return "UNKNOWN"
	}
	return strings.ToUpper(s)
}

// deriveState collapses the two append-only logs into a single display status.
//
// The decision log and the deploy log are independent streams that are never
// correlated per event; instead each owns the question it can answer. The
// decision log is authoritative for pre-deploy state (disabled, unhealthy,
// render/dependency failures); the deploy log is authoritative for the lifecycle
// of the last sync that was actually shipped. deriveState picks between them with
// a fixed precedence ladder rather than trying to match a decision to a deploy:
//
//  1. disabled-feature membership           -> DISABLED
//  2. deploy lifecycle in progress          -> SENT / INSTALLING   (a live sync; let it finish)
//  3. reconciler currently blocked          -> UNHEALTHY / MISSING-DEPS / MISSING-CONFIG / RENDER-ERROR (desired state cannot proceed)
//  4. terminal deploy outcome exists        -> DEPLOYED / FAILED    (the deploy log is the truth)
//  5. never produced a deploy yet           -> derive from the decision alone
//
// Rung 2 sits above rung 3 deliberately: an in-flight deploy is shown until it
// terminates even if the latest desired state no longer renders; once it
// terminates, rung 3 surfaces the blocker.
func deriveState(disabled bool, deploy feature.DeployStatus, action Action) string {
	switch {
	case disabled:
		return "disabled"
	case deploy.IsInProgress():
		return string(deploy) // sent / installing
	case action == ActionSkipUnhealthy:
		return "unhealthy"
	case action == ActionFailMissingDeps:
		return "missing-deps"
	case action == ActionFailMissingConfig:
		return "missing-config"
	case action == ActionFailRender:
		return "render-error"
	case deploy == feature.DeployStatusDeployed, deploy == feature.DeployStatusFailed:
		return string(deploy) // deployed / failed
	}

	// No deploy has ever shipped for this feature×environment; the decision is
	// the only signal.
	switch action {
	case ActionSkipDisabled:
		return "disabled"
	case ActionSkipInProgress, ActionDeploy:
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
	q := querier(ctx)
	decisions, err := q.ListDecisionStatuses(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list decision statuses: %w", err)
	}
	deploys, err := q.ListDeployStatuses(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list deploy statuses: %w", err)
	}
	disabled, err := q.ListDisabledEnvironments(ctx, featureAssignmentID)
	if err != nil {
		return nil, fmt.Errorf("list disabled environments: %w", err)
	}
	return joinReconcileSignals(featureAssignmentID, decisions, deploys, disabled), nil
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

// joinReconcileSignals joins the three raw per-environment signals by
// environment and derives the effective reconcile status for each.
func joinReconcileSignals(
	featureAssignmentID uuid.UUID,
	decisions []reconcilersql.ListDecisionStatusesRow,
	deploys []reconcilersql.ListDeployStatusesRow,
	disabled []reconcilersql.ListDisabledEnvironmentsRow,
) []*FeatureReconcileStatus {
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

		state := deriveState(isDisabled, feature.DeployStatus(dep.Status), Action(dec.Action))
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

	return statuses
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
