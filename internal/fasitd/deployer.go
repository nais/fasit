package fasitd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/fasitd/fasitdsql"
	"github.com/nais/fasit/internal/fasitd/protogen"
	"github.com/nais/fasit/internal/reconciler"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const sendTimeout = 5 * time.Second

// Deployer is the fasitd shadow lane of the reconciler. It mirrors each deploy
// decision as a fasitd command: it records the command, then either delivers it
// over an active session (status "sent") or records it as "undeliverable" when
// no session is connected. It never blocks the canonical naisd rollout.
type Deployer struct {
	registry *registry
	querier  fasitdsql.Querier
	log      *slog.Logger

	commands metric.Int64Counter
}

func NewDeployer(pool *pgxpool.Pool, srv *Server, meter metric.Meter, log *slog.Logger) (*Deployer, error) {
	commands, err := meter.Int64Counter(
		"fasitd_commands_total",
		metric.WithDescription("fasitd commands dispatched, by delivery outcome"),
	)
	if err != nil {
		return nil, fmt.Errorf("create commands counter: %w", err)
	}
	return &Deployer{
		registry: srv.Registry(),
		querier:  fasitdsql.New(pool),
		log:      log.With("subsystem", "fasitd-deployer"),
		commands: commands,
	}, nil
}

func (d *Deployer) Deploy(ctx context.Context, decisions []reconciler.DeployDecision) error {
	for _, dec := range decisions {
		if dec.Action != reconciler.ActionDeploy {
			continue
		}
		if err := d.dispatch(ctx, dec); err != nil {
			// Shadow lane is best-effort: log and continue so one failure does
			// not stall the rest of the cycle.
			d.log.With("err", err, "feature", dec.Feature.Name, "env", dec.EnvironmentName).Error("dispatch fasitd command")
		}
	}
	return nil
}

// Uninstall is a no-op in the shadow lane: fasitd runs dry-run only, so the
// canonical naisd path performs the actual uninstall.
func (d *Deployer) Uninstall(_ context.Context, _ uuid.UUID, _, envName, releaseName string) error {
	d.log.With("feature", releaseName, "env", envName).Info("dry-run: skipping uninstall in shadow lane")
	return nil
}

func (d *Deployer) dispatch(ctx context.Context, dec reconciler.DeployDecision) error {
	diid := uuid.New()
	vals, err := json.Marshal(dec.Values)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}

	if err := d.querier.CreateCommand(ctx, fasitdsql.CreateCommandParams{
		Diid:                diid,
		EnvironmentID:       dec.EnvironmentID,
		FeatureAssignmentID: dec.FeatureAssignmentID,
		FeatureName:         dec.Feature.Name,
		FeatureVersion:      dec.Feature.Version,
		Chart:               dec.Feature.Chart,
		ConfigHash:          dec.Hash,
		Uninstall:           false,
		Vals:                vals,
	}); err != nil {
		return fmt.Errorf("create command: %w", err)
	}

	sess, ok := d.registry.get(keyFor(dec.TenantName, dec.EnvironmentName))
	if !ok {
		d.record(ctx, dec, "undeliverable")
		return d.appendStatus(ctx, diid, "undeliverable", "no active fasitd session for environment")
	}

	cmd := &protogen.ServerMessage{
		Message: &protogen.ServerMessage_Command{
			Command: &protogen.Command{
				Diid:       diid.String(),
				Name:       dec.Feature.Name,
				Version:    dec.Feature.Version,
				Chart:      dec.Feature.Chart,
				ConfigHash: dec.Hash,
				TimeoutMs:  dec.Feature.Timeout.Milliseconds(),
				Values:     vals,
				Uninstall:  false,
			},
		},
	}

	select {
	case sess.send <- cmd:
		d.record(ctx, dec, "sent")
		return d.appendStatus(ctx, diid, "sent", "")
	case <-sess.done:
		d.record(ctx, dec, "undeliverable")
		return d.appendStatus(ctx, diid, "undeliverable", "fasitd session closed before delivery")
	case <-time.After(sendTimeout):
		d.record(ctx, dec, "undeliverable")
		return d.appendStatus(ctx, diid, "undeliverable", "timeout writing to fasitd session")
	}
}

func (d *Deployer) appendStatus(ctx context.Context, diid uuid.UUID, status, msg string) error {
	if err := d.querier.AppendCommandStatus(ctx, fasitdsql.AppendCommandStatusParams{
		Diid:    diid,
		Status:  status,
		Message: msg,
	}); err != nil {
		return fmt.Errorf("append command status: %w", err)
	}
	return nil
}

func (d *Deployer) record(ctx context.Context, dec reconciler.DeployDecision, outcome string) {
	if d.commands == nil {
		return
	}
	d.commands.Add(ctx, 1, metric.WithAttributes(
		attribute.String("outcome", outcome),
		attribute.String("tenant", dec.TenantName),
		attribute.String("environment", dec.EnvironmentName),
		attribute.String("feature", dec.Feature.Name),
	))
}
