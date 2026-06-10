package reconciler

import (
	"context"
	"log/slog"
)

// MultiDeployer fans deploy decisions out to a primary Deployer plus zero or
// more secondary best-effort deployers. The primary is canonical: its error
// fails the cycle. Secondary deployers (e.g. the fasitd shadow lane) run after
// the primary and their errors are logged, never propagated.
type MultiDeployer struct {
	primary     Deployer
	secondaries []Deployer
	log         *slog.Logger
}

func NewMultiDeployer(primary Deployer, log *slog.Logger, secondaries ...Deployer) *MultiDeployer {
	return &MultiDeployer{
		primary:     primary,
		secondaries: secondaries,
		log:         log.With("subsystem", "multi-deployer"),
	}
}

func (m *MultiDeployer) Deploy(ctx context.Context, decisions []DeployDecision) error {
	if err := m.primary.Deploy(ctx, decisions); err != nil {
		return err
	}
	for _, d := range m.secondaries {
		if err := d.Deploy(ctx, decisions); err != nil {
			m.log.With("err", err).Error("secondary deployer failed")
		}
	}
	return nil
}
