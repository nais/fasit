package reconciler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/dbtx"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"github.com/nais/fasit/internal/slack"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type Receiver struct {
	manager        ReceiverClient
	log            *slog.Logger
	slack          slack.SlackClient
	slackChannel   string
	messagesRecv   metric.Int64Counter
	deployDuration metric.Float64Histogram
	querier        reconcilersql.Querier
}

func NewReceiver(pool *pgxpool.Pool, mgr ReceiverClient, log *slog.Logger, slackClient slack.SlackClient, slackChannel string, meter metric.Meter) *Receiver {
	messagesRecv, err := meter.Int64Counter("status_messages_received_total",
		metric.WithDescription("Total status messages received from naisd, by type"),
	)
	if err != nil {
		log.With("err", err).Warn("failed to create status messages counter")
	}

	deployDuration, err := meter.Float64Histogram("deploy_instruction_duration_seconds",
		metric.WithDescription("Time from deploy instruction creation to terminal status"),
		metric.WithUnit("s"),
	)
	if err != nil {
		log.With("err", err).Warn("failed to create deploy duration histogram")
	}

	receiver := &Receiver{
		manager:        mgr,
		log:            log.With("subsystem", "status-receiver"),
		slack:          slackClient,
		slackChannel:   slackChannel,
		messagesRecv:   messagesRecv,
		deployDuration: deployDuration,
		querier:        reconcilersql.New(pool),
	}
	return receiver
}

func (r *Receiver) Run(ctx context.Context) {
	err := r.manager.Receive(ctx, r.handler)
	if err != nil {
		r.log.With("err", err).Error("receive status messages")
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
		r.log.With("type", msg.Type).Warn("unknown status type")
	}

	return nil
}

func (r *Receiver) handlerHelm(ctx context.Context, status message.Status) error {
	helmStatus := &message.Helm{}
	err := json.Unmarshal(status.Data, helmStatus)
	if err != nil {
		r.log.With("err", err).Error("invalid json")
		return nil
	}

	deploy, err := r.querier.LatestDeployByDIID(ctx, helmStatus.DIID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.With("diid", helmStatus.DIID).Warn("unknown deploy instruction")
			return nil
		}
		return err
	}

	env, err := envpkg.Get(ctx, deploy.EnvironmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.With("deploy_instruction", helmStatus.DIID).Warn("unknown deploy instruction")
			return nil
		}
		return err
	}

	if helmStatus.RolloutStatus == model.RolloutStatusFailed {
		tenant, err := envpkg.GetTenant(ctx, env.TenantID)
		if err != nil {
			return fmt.Errorf("getting tenant: %w", err)
		}

		slackMsg := r.slack.GetFeatureDeployFailedMessageOptions(deploy.FeatureName, tenant.Name, env.Name)
		if _, _, err := r.slack.PostMessage(r.slackChannel, slackMsg); err != nil {
			r.log.With("err", err).Error("sending slack message")
		}
	}

	// Append the terminal deploy_log row for this diid, carrying the hash forward
	// so deploy_status reflects the deployed hash for skip-unchanged comparison.
	if err := r.querier.AppendDeployStatus(ctx, reconcilersql.AppendDeployStatusParams{
		Diid:                helmStatus.DIID,
		EnvironmentID:       deploy.EnvironmentID,
		FeatureAssignmentID: deploy.FeatureAssignmentID,
		FeatureName:         deploy.FeatureName,
		FeatureVersion:      deploy.FeatureVersion,
		Status:              helmStatus.RolloutStatus.String(),
		Hash:                deploy.Hash,
	}); err != nil {
		return fmt.Errorf("appending deploy status: %w", err)
	}

	if r.deployDuration != nil {
		duration := time.Since(deploy.Created).Seconds()
		r.deployDuration.Record(ctx, duration, metric.WithAttributes(
			attribute.String("feature", deploy.FeatureName),
			attribute.String("status", helmStatus.RolloutStatus.String()),
		))
	}

	return nil
}

func (r *Receiver) releaseStatus(ctx context.Context, msg message.Status) error {
	status := &message.HelmRelease{}
	err := json.Unmarshal(msg.Data, status)
	if err != nil {
		r.log.With("err", err).Error("invalid json")
		return nil
	}

	t, err := envpkg.GetTenantByName(ctx, msg.Tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.With("tenant", msg.Tenant).Warn("unknown tenant")
		}
		return nil
	}
	env, err := envpkg.GetByName(ctx, t.ID, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.With("tenant", msg.Tenant, "environment", msg.Environment).Warn("unknown tenant and/or environment")
			return nil
		}
		return fmt.Errorf("getting environment id: %w", err)
	}

	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		err = r.querier.DeleteReleaseStatusesInEnvironment(ctx, env.ID)
		if err != nil {
			return fmt.Errorf("deleting release status: %w", err)
		}

		for _, rel := range status.Releases {
			err = r.querier.SetReleaseStatus(ctx, reconcilersql.SetReleaseStatusParams{
				EnvironmentID: env.ID,
				Feature:       rel.Name,
				Version:       rel.Version,
				Status:        rel.Status,
				Revision:      int32(rel.Revision), // #nosec G115
				LastDeployed:  rel.LastDeployed,
			})
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
		r.log.With("err", err).Error("invalid json")
		return nil
	}
	t, err := envpkg.GetTenantByName(ctx, msg.Tenant)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.With("tenant", msg.Tenant).Warn("unknown tenant")
		}
		return nil
	}
	env, err := envpkg.GetByName(ctx, t.ID, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.With("tenant", msg.Tenant, "environment", msg.Environment).Warn("unknown tenant and/or environment")
			return nil
		}
		return err
	}
	return naisdstatus.Set(ctx, env.ID, status)
}

func (r *Receiver) handleStatusLog(ctx context.Context, msg message.Status) error {
	status := &message.StatusLog{}

	if err := json.Unmarshal(msg.Data, status); err != nil {
		r.log.With("err", err).Error("invalid json")
		return nil
	}

	if err := feature.LogCreate(ctx, status.DIID, status.Logs); err != nil {
		r.log.With("err", err).Error("unable to log status")
	}

	return nil
}
