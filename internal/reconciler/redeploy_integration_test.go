//go:build integration_test

package reconciler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/reconciler"
)

func (h *reconcileTest) envID(tenant, env string) uuid.UUID {
	h.t.Helper()
	var id uuid.UUID
	err := h.pool.QueryRow(h.ctx, `
		SELECT e.id FROM environments e
		JOIN tenants t ON t.id = e.tenant_id
		WHERE t.name = $1 AND e.name = $2
	`, tenant, env).Scan(&id)
	if err != nil {
		h.t.Fatalf("lookup env id %s/%s: %v", tenant, env, err)
	}
	return id
}

func TestRedeploy(t *testing.T) {
	ctx := context.Background()
	container, dsn := startPostgresWithSnapshot(ctx, t)

	t.Run("forces a fresh deploy when settled and unchanged", func(t *testing.T) {
		h := newReconcileTest(ctx, t, container, dsn)
		h.reconciler.SetDeployer(h.deployer)
		h.createEnvs(tenantEnv{"nav", "dev", environment.Labels{}})
		h.createAssignment("naiserator", "1.0.0", environment.Labels{"tenant": "nav"})

		h.reconcile()
		h.requirePublished(1)
		h.reconcile() // unchanged: no new publish
		h.requirePublished(1)

		if err := h.reconciler.Redeploy(ctx, h.envID("nav", "dev"), "naiserator"); err != nil {
			t.Fatalf("redeploy: %v", err)
		}
		h.requirePublished(2)
		if got := h.countInstructions("naiserator", "1.0.0"); got != 2 {
			t.Errorf("distinct deploy instructions = %d, want 2", got)
		}
	})

	t.Run("refuses when feature is disabled", func(t *testing.T) {
		h := newReconcileTest(ctx, t, container, dsn)
		h.reconciler.SetDeployer(h.deployer)
		h.createEnvs(tenantEnv{"nav", "dev", environment.Labels{}})
		h.createAssignment("naiserator", "1.0.0", environment.Labels{"tenant": "nav"})
		h.reconcile()
		h.requirePublished(1)

		envID := h.envID("nav", "dev")
		if err := feature.DisableFeature(h.ctx, envID, "naiserator", "testing redeploy gate"); err != nil {
			t.Fatalf("disable feature: %v", err)
		}

		err := h.reconciler.Redeploy(ctx, envID, "naiserator")
		if !errors.Is(err, reconciler.ErrRedeployNotSettled) {
			t.Fatalf("redeploy err = %v, want ErrRedeployNotSettled", err)
		}
		h.requirePublished(1) // nothing new published
	})

	t.Run("refuses when no deployment matches", func(t *testing.T) {
		h := newReconcileTest(ctx, t, container, dsn)
		h.reconciler.SetDeployer(h.deployer)
		h.createEnvs(tenantEnv{"nav", "dev", environment.Labels{}})

		err := h.reconciler.Redeploy(ctx, h.envID("nav", "dev"), "naiserator")
		if !errors.Is(err, reconciler.ErrRedeployTargetNotFound) {
			t.Fatalf("redeploy err = %v, want ErrRedeployTargetNotFound", err)
		}
	})
}
