package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/slack"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type HelmListener interface {
	Receive(ctx context.Context, message *message.Helm) error
}

type Receiver struct {
	manager        ReceiverClient
	log            *slog.Logger
	slack          slack.SlackClient
	slackChannel   string
	listeners      []HelmListener
	messagesRecv   metric.Int64Counter
	deployDuration metric.Float64Histogram
}

func NewReceiver(
	mgr ReceiverClient,
	log *slog.Logger,
	slackClient slack.SlackClient,
	slackChannel string,
	meter metric.Meter,
	listeners ...HelmListener,
) *Receiver {
	messagesRecv, err := meter.Int64Counter("status_messages_received_total",
		metric.WithDescription("Total status messages received from naisd, by type"),
	)
	if err != nil {
		log.Warn("failed to create status messages counter", "error", err)
	}

	deployDuration, err := meter.Float64Histogram("deploy_instruction_duration_seconds",
		metric.WithDescription("Time from deploy instruction creation to terminal status"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.Warn("failed to create deploy duration histogram", "error", err)
	}

	receiver := &Receiver{
		manager:        mgr,
		log:            log.With("subsystem", "status-receiver"),
		slack:          slackClient,
		slackChannel:   slackChannel,
		listeners:      listeners,
		messagesRecv:   messagesRecv,
		deployDuration: deployDuration,
	}
	return receiver
}

func (r *Receiver) Run(ctx context.Context) {
	err := r.manager.Receive(ctx, r.handler)
	if err != nil {
		r.log.Error("receive status messages", "error", err)
	}
}

func (r *Receiver) handler(ctx context.Context, msg message.Status) error {
	if r.messagesRecv != nil {
		r.messagesRecv.Add(ctx, 1, metric.WithAttributes(
			attribute.String("type", msg.Type.String()),
			attribute.String("tenant", msg.Tenant),
			attribute.String("environment", msg.Environment),
		))
	}

	switch msg.Type {
	case message.StatusTypeHelm:
		return r.handlerHelm(ctx, msg)
	case message.StatusTypeHelmReleases:
		return r.releaseStatus(ctx, msg)
	case message.StatusTypeHealth:
		return r.healthStatus(ctx, msg)
	case message.StatusTypeLog:
		return r.handleStatusLog(ctx, msg)
	default:
		r.log.Warn("unknown status type", "type", msg.Type)
	}

	return nil
}

func (r *Receiver) handlerHelm(ctx context.Context, msg message.Status) error {
	helmStatus := &message.Helm{}
	err := json.Unmarshal(msg.Data, helmStatus)
	if err != nil {
		r.log.Error("invalid json", "error", err)
		return nil
	}

	for _, l := range r.listeners {
		if err := l.Receive(ctx, helmStatus); err != nil {
			r.log.Error("notifying helm listener", "error", err)
		}
	}

	di, err := featureassignment.GetDeployInstruction(ctx, helmStatus.DIID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Warn("unknown deploy instruction", "diid", helmStatus.DIID)
			return nil
		}
		return err
	}

	env, err := environment.Get(ctx, di.EnvironmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Warn("unknown deploy instruction", "deploy_instruction", helmStatus.DIID)
			return nil
		}
		return err
	}

	if helmStatus.RolloutStatus == model.RolloutStatusFailed {
		tenant, err := environment.GetTenant(ctx, env.TenantID)
		if err != nil {
			return fmt.Errorf("getting tenant: %w", err)
		}

		slackMsg := r.slack.GetFeatureDeployFailedMessageOptions(di.FeatureName, tenant.Name, env.Name)
		if _, _, err := r.slack.PostMessage(r.slackChannel, slackMsg); err != nil {
			r.log.Error("sending slack message", "error", err)
		}
	}

	if err := featureassignment.UpdateDeployInstructionStatus(ctx, helmStatus.DIID, helmStatus.RolloutStatus); err != nil {
		return fmt.Errorf("updating deploy instruction status: %w", err)
	}

	if r.deployDuration != nil {
		duration := time.Since(di.Created).Seconds()
		r.deployDuration.Record(ctx, duration, metric.WithAttributes(
			attribute.String("feature", di.FeatureName),
			attribute.String("status", helmStatus.RolloutStatus.String()),
		))
	}

	return nil
}

func (r *Receiver) releaseStatus(ctx context.Context, msg message.Status) error {
	status := &message.HelmRelease{}
	err := json.Unmarshal(msg.Data, status)
	if err != nil {
		r.log.Error("invalid json", "error", err)
		return nil
	}

	t, err := environment.GetTenantByName(ctx, msg.Tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Warn("unknown tenant", "tenant", msg.Tenant)
		}
		return nil
	}
	env, err := environment.GetByName(ctx, t.ID, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Warn("unknown tenant and/or environment", "tenant", msg.Tenant, "environment", msg.Environment)
			return nil
		}
		return fmt.Errorf("getting environment id: %w", err)
	}

	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err = featureassignment.DeleteReleaseStatus(ctx, env.ID)
		if err != nil {
			return fmt.Errorf("deleting release status: %w", err)
		}

		for _, rel := range status.Releases {
			err = featureassignment.SetReleaseStatus(ctx, env.ID, &rel)
			if err != nil {
				return fmt.Errorf("creating release status: %w", err)
			}
		}
		return nil
	})
}

func (r *Receiver) healthStatus(ctx context.Context, msg message.Status) error {
	status := &message.Health{}
	err := json.Unmarshal(msg.Data, status)
	if err != nil {
		r.log.Error("invalid json", "error", err)
		return nil
	}
	t, err := environment.GetTenantByName(ctx, msg.Tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Warn("unknown tenant", "tenant", msg.Tenant)
		}
		return nil
	}
	env, err := environment.GetByName(ctx, t.ID, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.Warn("unknown tenant and/or environment", "tenant", msg.Tenant, "environment", msg.Environment)
			return nil
		}
		return err
	}
	return naisdstatus.Set(ctx, env.ID, status)
}

func (r *Receiver) handleStatusLog(ctx context.Context, msg message.Status) error {
	status := &message.StatusLog{}

	if err := json.Unmarshal(msg.Data, status); err != nil {
		r.log.Error("invalid json", "error", err)
		return nil
	}

	if err := feature.LogCreate(ctx, status.DIID, status.Logs); err != nil {
		r.log.Error("unable to log status", "error", err)
	}

	return nil
}
