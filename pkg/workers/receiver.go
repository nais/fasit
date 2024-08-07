package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/slack"
	"github.com/sirupsen/logrus"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type ReceiverStore interface {
	DeployInstructionGet(ctx context.Context, id uuid.UUID) (*model.DeployInstruction, error)
	DeployInstructionsLatestForFeature(ctx context.Context, envID uuid.UUID, featureName string) (*model.DeployInstruction, error)
	DeployInstructionUpdateStatus(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error
	EnvironmentByNames(ctx context.Context, tenantName, environmentName string) (*model.Environment, error)
	EnvironmentCI(ctx context.Context, kind model.EnvironmentKind) (*model.Environment, error)
	EnvironmentCreate(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error)
	EnvironmentGet(ctx context.Context, id uuid.UUID) (*model.Environment, error)
	EnvironmentIDByNames(ctx context.Context, tenantName string, environmentName string) (uuid.UUID, error)
	FeatureVersionUpdate(ctx context.Context, name string, version string) error
	HealthStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Health) error
	KubernetesNodeSync(ctx context.Context, envID uuid.UUID, kn *message.KubernetesNodes) error
	LogCreate(ctx context.Context, deployInstructionID uuid.UUID, lines []message.LogLine) error
	ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error
	RolloutByName(ctx context.Context, name string) (*model.Feature, error)
	RolloutCalculateDone(ctx context.Context, rolloutID uuid.UUID) (bool, error)
	RolloutDelete(ctx context.Context, name string) error
	RolloutEventCreate(ctx context.Context, rollout uuid.UUID, failure bool, message string, data map[string]any) error
	RolloutStatus(ctx context.Context, name string) (model.RolloutStatus, error)
	RolloutsUpdateStatus(ctx context.Context, status model.RolloutStatus, name string, completed bool) error
	TenantCreate(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error)
	TenantGet(ctx context.Context, id uuid.UUID) (*model.Tenant, error)
	TenantGetByName(ctx context.Context, name string) (*model.Tenant, error)
	TxFunc(ctx context.Context, fn database.TXFunc) error
}

type Receiver struct {
	manager      ReceiverClient
	repo         ReceiverStore
	log          *logrus.Entry
	slack        slack.SlackClient
	slackChannel string
}

func NewReceiver(mgr ReceiverClient, repo ReceiverStore, log *logrus.Entry, slackClient slack.SlackClient, slackChannel string) *Receiver {
	receiver := &Receiver{
		manager:      mgr,
		repo:         repo,
		log:          log,
		slack:        slackClient,
		slackChannel: slackChannel,
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
	case message.StatusKubernetesNodes:
		return r.kubernetesNodes(ctx, msg)
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

	di, err := r.repo.DeployInstructionGet(ctx, helmStatus.DIID)
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
		tenant, err := r.repo.TenantGet(ctx, env.TenantID)
		if err != nil {
			return fmt.Errorf("getting tenant: %w", err)
		}

		slackMsg := r.slack.GetFeatureDeployFailed(di.FeatureName, tenant.Name, env.Name)
		if _, _, err := r.slack.PostMessage(r.slackChannel, slackMsg); err != nil {
			r.log.WithError(err).Error("sending slack message")
		}
	}

	if err := r.repo.DeployInstructionUpdateStatus(ctx, helmStatus.DIID, helmStatus.RolloutStatus); err != nil {
		return fmt.Errorf("updating deploy instruction status: %w", err)
	}

	if env.CI {
		if err := r.handleCI(ctx, env, di, helmStatus, msg.Tenant); err != nil {
			r.log.WithError(err).Error("handling helm status message for CI environment")
		}
	}

	return nil
}

func (r *Receiver) handleCI(ctx context.Context, env *model.Environment, di *model.DeployInstruction, helmStatus *message.Helm, tenant string) error {
	featureName := di.FeatureName
	featureVersion := di.FeatureVersion

	rollout, err := r.repo.RolloutByName(ctx, featureName)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("getting rollout: %w", err)
		} else {
			r.log.WithField("name", featureName).Debug("not part of a rollout")
			return nil
		}
	}

	if rollout.Version != featureVersion {
		r.log.WithField("name", featureName).Warn("version mismatch")
		return nil
	}

	eventData := map[string]any{
		"environment": env.Name,
		"tenant":      tenant,
	}

	status := model.RolloutStatusUnknown
	switch helmStatus.RolloutStatus {
	case model.RolloutStatusPending:
		if err := r.repo.RolloutEventCreate(ctx, rollout.GraphVars.RolloutID, false, "Installing helm chart...", eventData); err != nil {
			return fmt.Errorf("creating rollout event: %w", err)
		}
		// We have nothing more to do here as it's still pending
		return nil
	case model.RolloutStatusFailed:
		status = model.RolloutStatusFailed
		if err := r.repo.RolloutEventCreate(ctx, rollout.GraphVars.RolloutID, true, "Helm install failed", eventData); err != nil {
			return fmt.Errorf("creating rollout event: %w", err)
		}
	case model.RolloutStatusDeployed:
		status = model.RolloutStatusDeployed
		if err := r.repo.RolloutEventCreate(ctx, rollout.GraphVars.RolloutID, false, "Helm install succeeded", eventData); err != nil {
			return fmt.Errorf("creating rollout event: %w", err)
		}
	default:
		return fmt.Errorf("invalid helm status: %v", helmStatus.RolloutStatus)
	}

	// At this point the rollout is either failed or deployed
	return r.repo.TxFunc(ctx, func(repo database.Repo) error {
		last, err := r.repo.RolloutCalculateDone(ctx, rollout.GraphVars.RolloutID)
		if err != nil {
			return fmt.Errorf("checking if last: %w", err)
		}
		if !last {
			r.log.WithField("name", featureName).Debug("not last rollout for ci")
			return nil
		}

		r.log.WithField("name", featureName).Debug("last rollout for ci")

		// Calculate proper rollout status
		rolloutStatus, err := r.repo.RolloutStatus(ctx, rollout.Name)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				r.log.WithField("name", featureName).Warn("unknown rollout")
				return nil
			}
			return fmt.Errorf("getting rollout status for %q: %w", rollout.Name, err)
		}

		if status == model.RolloutStatusFailed || rolloutStatus == model.RolloutStatusFailed {
			if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusFailed, rollout.Name, true); err != nil {
				return fmt.Errorf("updating rollout status (failed): %w", err)
			}
			r.log.WithField("name", featureName).Info("rollout failed")
			return nil
		}

		if err := repo.FeatureVersionUpdate(ctx, rollout.Name, rollout.Version); err != nil {
			return fmt.Errorf("updating feature version: %w", err)
		}

		if len(rollout.Rename) > 0 {
			if err := repo.ConfigMove(ctx, rollout.Name, rollout.Rename); err != nil {
				return fmt.Errorf("moving config: %w", err)
			}
		}

		if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, rollout.Name, true); err != nil {
			return fmt.Errorf("updating rollout status: %w", err)
		}

		r.log.WithField("name", featureName).Info("rollout succeeded")
		return nil
	})
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
	return r.repo.HealthStatusCreateOrUpdate(ctx, environmentID, status)
}

func (r *Receiver) kubernetesNodes(ctx context.Context, msg message.Status) error {
	status := &message.KubernetesNodes{}
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

	return r.repo.KubernetesNodeSync(ctx, environmentID, status)
}

func (r *Receiver) handleStatusLog(ctx context.Context, msg message.Status) error {
	status := &message.StatusLog{}

	if err := json.Unmarshal(msg.Data, status); err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}

	return r.repo.LogCreate(ctx, status.DIID, status.Logs)
}
