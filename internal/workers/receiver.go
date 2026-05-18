package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/slack"
	"github.com/sirupsen/logrus"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type HelmListener interface {
	Receive(ctx context.Context, message *message.Helm) error
}

type ReceiverStore interface {
	DeployInstructionsLatestForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error)
	DeployInstructionUpdateStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error
	EnvironmentByNames(ctx context.Context, tenantName, environmentName string) (*model.Environment, error)
	EnvironmentCI(ctx context.Context, kind model.EnvironmentKind) (*model.Environment, error)
	EnvironmentCreate(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error)
	EnvironmentGet(ctx context.Context, id uuid.UUID) (*model.Environment, error)
	EnvironmentIDByNames(ctx context.Context, tenantName string, environmentName string) (uuid.UUID, error)
	ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error
	TxFunc(ctx context.Context, fn database.TXFunc) error
}

type Receiver struct {
	manager      ReceiverClient
	repo         ReceiverStore
	log          logrus.FieldLogger
	slack        slack.SlackClient
	slackChannel string
	listeners    []HelmListener
}

func NewReceiver(
	mgr ReceiverClient,
	repo ReceiverStore,
	log logrus.FieldLogger,
	slackClient slack.SlackClient,
	slackChannel string,
	listeners ...HelmListener,
) *Receiver {
	receiver := &Receiver{
		manager:      mgr,
		repo:         repo,
		log:          log.WithField("subsystem", "status-receiver"),
		slack:        slackClient,
		slackChannel: slackChannel,
		listeners:    listeners,
	}
	return receiver
}

func (r *Receiver) Run(ctx context.Context) {
	err := r.manager.Receive(ctx, r.handler)
	if err != nil {
		r.log.WithError(err).Error("receive status messages")
	}
}

func (r *Receiver) handler(ctx context.Context, msg message.Status) error {
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
		r.log.WithField("type", msg.Type).Warn("unknown status type")
	}

	return nil
}

func (r *Receiver) handlerHelm(ctx context.Context, msg message.Status) error {
	helmStatus := &message.Helm{}
	err := json.Unmarshal(msg.Data, helmStatus)
	if err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	for _, l := range r.listeners {
		if err := l.Receive(ctx, helmStatus); err != nil {
			r.log.WithError(err).Error("notifying helm listener")
		}
	}

	di, err := deployment.GetDeployInstructionByID(ctx, helmStatus.DIID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.WithField("diid", helmStatus.DIID).Warn("unknown deploy instruction")
			return nil
		}
		return err
	}

	env, err := r.repo.EnvironmentGet(ctx, di.EnvironmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.WithField("deploy_instruction", helmStatus.DIID).
				Warn("unknown deploy instruction")
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
			r.log.WithError(err).Error("sending slack message")
		}
	}

	if err := r.repo.DeployInstructionUpdateStatus(ctx, helmStatus.DIID, helmStatus.RolloutStatus); err != nil {
		return fmt.Errorf("updating deploy instruction status: %w", err)
	}

	return nil
}

func (r *Receiver) releaseStatus(ctx context.Context, msg message.Status) error {
	status := &message.HelmRelease{}
	err := json.Unmarshal(msg.Data, status)
	if err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	environmentID, err := r.repo.EnvironmentIDByNames(ctx, msg.Tenant, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.WithField("tenant", msg.Tenant).
				WithField("environment", msg.Environment).
				Warn("unknown tenant and/or environment")
			return nil
		}
		return fmt.Errorf("getting environment id: %w", err)
	}

	return r.repo.TxFunc(ctx, func(repo database.Repo) error {
		err = repo.ReleaseStatusDeleteByEnvironmentID(ctx, environmentID)
		if err != nil {
			return fmt.Errorf("deleting release status: %w", err)
		}

		for _, rel := range status.Releases {
			err = repo.ReleaseStatusCreateOrUpdate(ctx, environmentID, &rel)
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
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}
	environmentID, err := r.repo.EnvironmentIDByNames(ctx, msg.Tenant, msg.Environment)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			r.log.WithField("tenant", msg.Tenant).
				WithField("environment", msg.Environment).
				Warn("unknown tenant and/or environment")
			return nil
		}
		return err
	}
	return naisdstatus.Set(ctx, environmentID, status)
}

func (r *Receiver) handleStatusLog(ctx context.Context, msg message.Status) error {
	status := &message.StatusLog{}

	if err := json.Unmarshal(msg.Data, status); err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	if err := feature.LogCreate(ctx, status.DIID, status.Logs); err != nil {
		r.log.WithError(err).Errorf("unable to log status")
	}

	return nil
}
