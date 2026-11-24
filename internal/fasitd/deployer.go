package fasitd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/contextloader"
	envpkg "github.com/nais/fasit/internal/environment"
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
	registry    *registry
	querier     fasitdsql.Querier
	loadContext contextloader.LoaderFunc
	log         *slog.Logger

	commands metric.Int64Counter
}

func NewDeployer(pool *pgxpool.Pool, srv *Server, loadContext contextloader.LoaderFunc, meter metric.Meter, log *slog.Logger) (*Deployer, error) {
	commands, err := meter.Int64Counter(
		"fasitd_commands_total",
		metric.WithDescription("fasitd commands dispatched, by delivery outcome"),
	)
	if err != nil {
		return nil, fmt.Errorf("create commands counter: %w", err)
	}
	return &Deployer{
		registry:    srv.Registry(),
		querier:     fasitdsql.New(pool),
		loadContext: loadContext,
		log:         log.With("subsystem", "fasitd-deployer"),
		commands:    commands,
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

func (d *Deployer) Uninstall(ctx context.Context, diid uuid.UUID, tenantName, envName, releaseName string) error {
	reqCtx := d.loadContext(ctx)
	tenant, err := envpkg.GetTenantByName(reqCtx, tenantName)
	if err != nil {
		return fmt.Errorf("get tenant %s: %w", tenantName, err)
	}
	env, err := envpkg.GetByName(reqCtx, tenant.ID, envName)
	if err != nil {
		return fmt.Errorf("get environment %s/%s: %w", tenantName, envName, err)
	}

	if err := d.querier.CreateCommand(ctx, fasitdsql.CreateCommandParams{
		Diid:          diid,
		EnvironmentID: env.ID,
		FeatureName:   releaseName,
		Uninstall:     true,
		Vals:          []byte("{}"),
	}); err != nil {
		return fmt.Errorf("create uninstall command: %w", err)
	}

	cmd := &protogen.ServerMessage{
		Message: &protogen.ServerMessage_Command{
			Command: &protogen.Command{
				Diid:      diid.String(),
				Name:      releaseName,
				Uninstall: true,
			},
		},
	}

	return d.sendCommand(ctx, diid, tenantName, envName, releaseName, cmd)
}

func (d *Deployer) dispatch(ctx context.Context, dec reconciler.DeployDecision) error {
	diid := uuid.New()
	vals, err := json.Marshal(dec.Values)
	if err != nil {
		return fmt.Errorf("marshal values: %w", err)
	}

	faID := dec.FeatureAssignmentID
	if err := d.querier.CreateCommand(ctx, fasitdsql.CreateCommandParams{
		Diid:                diid,
		EnvironmentID:       dec.EnvironmentID,
		FeatureAssignmentID: &faID,
		FeatureName:         dec.Feature.Name,
		FeatureVersion:      dec.Feature.Version,
		Chart:               dec.Feature.Chart,
		ConfigHash:          dec.Hash,
		Uninstall:           false,
		Vals:                vals,
	}); err != nil {
		return fmt.Errorf("create command: %w", err)
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

	return d.sendCommand(ctx, diid, dec.TenantName, dec.EnvironmentName, dec.Feature.Name, cmd)
}

func (d *Deployer) sendCommand(ctx context.Context, diid uuid.UUID, tenantName, envName, featureName string, cmd *protogen.ServerMessage) error {
	sessions := d.registry.list(keyFor(tenantName, envName))
	if len(sessions) == 0 {
		d.record(ctx, tenantName, envName, featureName, "undeliverable")
		return d.appendStatus(ctx, diid, "undeliverable", "no active fasitd session for environment")
	}

	lastReason := "no active fasitd session for environment"
	for _, sess := range sessions {
		select {
		case sess.send <- cmd:
			d.record(ctx, tenantName, envName, featureName, "sent")
			return d.appendStatus(ctx, diid, "sent", "")
		case <-sess.done:
			d.registry.remove(sess)
			lastReason = "fasitd session closed before delivery"
		case <-time.After(sendTimeout):
			d.registry.remove(sess)
			lastReason = "timeout writing to fasitd session"
		}
	}

	d.record(ctx, tenantName, envName, featureName, "undeliverable")
	return d.appendStatus(ctx, diid, "undeliverable", lastReason)
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

func (d *Deployer) record(ctx context.Context, tenantName, envName, featureName, outcome string) {
	if d.commands == nil {
		return
	}
	d.commands.Add(ctx, 1, metric.WithAttributes(
		attribute.String("outcome", outcome),
		attribute.String("tenant", tenantName),
		attribute.String("environment", envName),
		attribute.String("feature", featureName),
	))
}
