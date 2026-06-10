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
//  1. reconciler skipped because disabled   -> DISABLED
//  2. deploy lifecycle in progress          -> SENT / INSTALLING   (a live sync; let it finish)
//  3. reconciler currently blocked          -> UNHEALTHY / MISSING-DEPS / MISSING-CONFIG / RENDER-ERROR (desired state cannot proceed)
//  4. terminal deploy outcome exists        -> DEPLOYED / FAILED    (the deploy log is the truth)
//  5. never produced a deploy yet           -> derive from the decision alone
//
// Disabled (rung 1) is read from the latest decision (ActionSkipDisabled) rather
// than a separate disabled-features lookup: the reconciler only decides for the
// winning assignment per environment, so keying off the decision keeps the
// signal correctly scoped to that winner. Disabling triggers a reconcile, so the
// decision log reflects it promptly. Rung 1 sits above the terminal deploy
// outcome (rung 4) so a disable surfaces over a prior successful deploy.
//
// Rung 2 sits above rung 3 deliberately: an in-flight deploy is shown until it
// terminates even if the latest desired state no longer renders; once it
// terminates, rung 3 surfaces the blocker.
func deriveState(deploy feature.DeployStatus, action Action) string {
	switch {
	case action == ActionSkipDisabled:
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
	case ActionSkipInProgress, ActionDeploy:
		return "pending"
	case ActionSkipUnchanged:
		return "deployed"
	default:
		return "unknown"
	}
}

// decisionSignal and deploySignal are the per-environment inputs to
// joinReconcileSignals, decoupled from the generated row types so the
// single-assignment and batch read paths can share the join logic.
type decisionSignal struct {
	envID   uuid.UUID
	action  Action
	message string
	created time.Time
}

type deploySignal struct {
	envID   uuid.UUID
	status  string
	created time.Time
}

// ReconcileStatuses returns the effective reconcile status per environment for a
// feature assignment. It reads the latest decision and deploy rollout state per
// environment and joins them in Go.
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

	ds := make([]decisionSignal, len(decisions))
	for i, d := range decisions {
		ds[i] = decisionSignal{envID: d.EnvironmentID, action: Action(d.Action), message: d.Message, created: d.Created}
	}
	dp := make([]deploySignal, len(deploys))
	for i, d := range deploys {
		dp[i] = deploySignal{envID: d.EnvironmentID, status: d.Status, created: d.Created}
	}
	return joinReconcileSignals(featureAssignmentID, ds, dp), nil
}

// AllReconcileStatuses returns the effective reconcile status per environment for
// every feature assignment, keyed by feature assignment id. It reads the
// decision and deploy state for all assignments in two queries and joins them in
// Go, avoiding the per-assignment query fan-out of calling ReconcileStatuses in
// a loop.
func AllReconcileStatuses(ctx context.Context) (map[uuid.UUID][]*FeatureReconcileStatus, error) {
	q := querier(ctx)
	decisions, err := q.ListAllDecisionStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all decision statuses: %w", err)
	}
	deploys, err := q.ListAllDeployStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list all deploy statuses: %w", err)
	}

	decByFA := make(map[uuid.UUID][]decisionSignal)
	for _, d := range decisions {
		decByFA[d.FeatureAssignmentID] = append(decByFA[d.FeatureAssignmentID],
			decisionSignal{envID: d.EnvironmentID, action: Action(d.Action), message: d.Message, created: d.Created})
	}
	depByFA := make(map[uuid.UUID][]deploySignal)
	for _, d := range deploys {
		depByFA[d.FeatureAssignmentID] = append(depByFA[d.FeatureAssignmentID],
			deploySignal{envID: d.EnvironmentID, status: d.Status, created: d.Created})
	}

	out := make(map[uuid.UUID][]*FeatureReconcileStatus, len(decByFA))
	for faID, ds := range decByFA {
		out[faID] = joinReconcileSignals(faID, ds, depByFA[faID])
	}
	for faID, dp := range depByFA {
		if _, ok := out[faID]; !ok {
			out[faID] = joinReconcileSignals(faID, nil, dp)
		}
	}
	return out, nil
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
	decisions []decisionSignal,
	deploys []deploySignal,
) []*FeatureReconcileStatus {
	decByEnv := make(map[uuid.UUID]decisionSignal, len(decisions))
	for _, d := range decisions {
		decByEnv[d.envID] = d
	}
	depByEnv := make(map[uuid.UUID]deploySignal, len(deploys))
	for _, d := range deploys {
		depByEnv[d.envID] = d
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
	sort.Slice(envIDs, func(i, j int) bool { return envIDs[i].String() < envIDs[j].String() })

	statuses := make([]*FeatureReconcileStatus, len(envIDs))
	for i, envID := range envIDs {
		dec := decByEnv[envID]
		dep := depByEnv[envID]

		state := deriveState(feature.DeployStatus(dep.status), dec.action)
		lastModified := latestTime(dec.created, dep.created)
		statuses[i] = &FeatureReconcileStatus{
			State:               FeatureReconcileStatusState(NormalizeStatus(state)),
			Message:             dec.message,
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
