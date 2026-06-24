package reconciler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrRedeployNotSettled is returned when a redeploy is requested for a
// feature×environment that is not in a settled, deployable state (e.g. a
// deploy is in progress, the feature is disabled, the environment is
// unhealthy, or the last decision failed). The decision's message is wrapped
// to explain why.
var ErrRedeployNotSettled = errors.New("feature is not in a redeployable state")

// ErrRedeployTargetNotFound is returned when no reconciling environment or
// winning assignment matches the given environment and feature.
var ErrRedeployTargetNotFound = errors.New("no reconciling deployment found for feature in environment")

// ErrUninstallProtected is returned when an uninstall is requested for a
// release that must not be removed, such as the agents (naisd, fasitd) that
// execute the uninstall itself.
var ErrUninstallProtected = errors.New("release is a managed agent and cannot be uninstalled")

// protectedReleases must never be uninstalled: they are the agents that carry
// out the uninstall, so removing them would sever Fasit's control of the
// environment. The agent enforces this too; this is a server-side backstop.
var protectedReleases = map[string]bool{
	"naisd":  true,
	"fasitd": true,
}

// Uninstall asks the environment's naisd agent to remove releaseName. It is a
// deliberate operator action for cleaning up releases that no longer have a
// matching feature assignment; the reconciler never uninstalls on its own.
func (r *Reconciler) Uninstall(ctx context.Context, tenantName, envName, releaseName string) (uuid.UUID, error) {
	if r.deployer == nil {
		return uuid.Nil, errors.New("reconciler has no deployer configured")
	}
	if releaseName == "" || protectedReleases[releaseName] {
		return uuid.Nil, ErrUninstallProtected
	}
	diid := uuid.New()
	if err := r.deployer.Uninstall(ctx, diid, tenantName, envName, releaseName); err != nil {
		return uuid.Nil, err
	}
	return diid, nil
}

// Redeploy forces a fresh deploy of featureName to the given environment even
// though its rendered config is unchanged. It only proceeds when the feature
// is in a settled state (ActionSkipUnchanged); otherwise it returns
// ErrRedeployNotSettled wrapping the reason. The deploy reuses the normal
// reconcile path, so it appends a standard deploy_log row and publishes to
// naisd exactly like a real change.
func (r *Reconciler) Redeploy(ctx context.Context, envID uuid.UUID, featureName string) error {
	if r.deployer == nil {
		return errors.New("reconciler has no deployer configured")
	}

	snap, err := r.fetchSnapshot(ctx)
	if err != nil {
		return fmt.Errorf("fetch snapshot: %w", err)
	}

	var env *environment
	for i := range snap.environments {
		if snap.environments[i].ID == envID {
			env = &snap.environments[i]
			break
		}
	}
	if env == nil {
		return ErrRedeployTargetNotFound
	}

	winners := mostSpecificPerFeature(matchAssignments(snap.assignments, *env))
	var dep *reconcileAssignment
	for _, w := range winners {
		if w.Feature.Name == featureName {
			dep = w
			break
		}
	}
	if dep == nil {
		return ErrRedeployTargetNotFound
	}

	// Mirror the dispatch-level gates that computeAction does not perform.
	if reportedAt, ok := snap.healthByEnv[env.ID]; !ok || time.Since(reportedAt) > 3*time.Minute {
		return fmt.Errorf("%w: naisd is unhealthy", ErrRedeployNotSettled)
	}
	if snap.disabledByEnv[env.ID][featureName] {
		return fmt.Errorf("%w: feature reconcile disabled", ErrRedeployNotSettled)
	}

	decision := r.computeAction(snap, *env, dep)
	if decision.Action != ActionSkipUnchanged {
		return fmt.Errorf("%w: %s", ErrRedeployNotSettled, decision.Message)
	}

	decision.Action = ActionDeploy
	decision.Message = "redeploy requested"
	if err := r.deployer.Deploy(ctx, []DeployDecision{decision}); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}
	return nil
}
