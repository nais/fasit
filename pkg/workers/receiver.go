package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type ReceiverStore interface {
	EnvironmentByNames(ctx context.Context, tenantName, environmentName string) (*model.Environment, error)
	EnvironmentCI(ctx context.Context, kind model.EnvironmentKind) (*model.Environment, error)
	EnvironmentCreate(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error)
	EnvironmentIDByNames(ctx context.Context, tenantName string, environmentName string) (uuid.UUID, error)
	FeatureVersionUpdate(ctx context.Context, name string, version string) error
	HealthStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Health) error
	KubernetesNodeSync(ctx context.Context, envID uuid.UUID, kn *message.KubernetesNodes) error
	ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error
	RolloutByName(ctx context.Context, name string) (*model.Feature, error)
	RolloutDelete(ctx context.Context, name string) error
	RolloutEventCreate(ctx context.Context, rollout uuid.UUID, failure bool, message string) error
	RolloutStatus(ctx context.Context, name string) (model.RolloutStatus, error)
	RolloutsUpdateStatus(ctx context.Context, status model.RolloutStatus, name string, completed bool) error
	StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Helm) error
	StatusForFeature(ctx context.Context, environmentID uuid.UUID, feature string) (*model.Status, error)
	TenantCreate(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error)
	TenantGetByName(ctx context.Context, name string) (*model.Tenant, error)
	TxFunc(ctx context.Context, fn database.TXFunc) error
}

type Receiver struct {
	manager ReceiverClient
	repo    ReceiverStore
	log     *logrus.Entry
}

func NewReceiver(mgr ReceiverClient, repo ReceiverStore, log *logrus.Entry) *Receiver {
	receiver := &Receiver{
		manager: mgr,
		repo:    repo,
		log:     log,
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

	env, err := r.repo.EnvironmentByNames(ctx, msg.Tenant, msg.Environment)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.WithField("tenant", msg.Tenant).
				WithField("environment", msg.Environment).
				Warn("unknown tenant and/or environment")
			return nil
		}
		return err
	}

	if env.CI {
		if err := r.handleCI(ctx, env, helmStatus); err != nil {
			r.log.WithError(err).Error("handling helm status message for CI environment")
		}
	}

	err = r.repo.StatusCreateOrUpdate(ctx, env.ID, helmStatus)
	if err != nil {
		return err
	}

	return nil
}

func (r *Receiver) handleCI(ctx context.Context, env *model.Environment, helmStatus *message.Helm) error {
	rollout, err := r.repo.RolloutByName(ctx, helmStatus.Name)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return fmt.Errorf("getting rollout: %w", err)
		} else {
			r.log.WithField("name", helmStatus.Name).Debug("not part of a rollout")
			return nil
		}
	}

	if rollout.Version != helmStatus.Version {
		r.log.WithField("name", helmStatus.Name).Warn("version mismatch")
		return nil
	}

	// At this point, we know the status message is for a active rollout
	switch helmStatus.RolloutStatus {
	case model.RolloutStatusPending:
		if err := r.repo.RolloutEventCreate(ctx, rollout.GraphVars.RolloutID, false, "Helm installing..."); err != nil {
			return fmt.Errorf("creating rollout event: %w", err)
		}
		return nil
	case model.RolloutStatusFailed:
		if err := r.repo.RolloutsUpdateStatus(ctx, model.RolloutStatusFailed, rollout.Name, false); err != nil {
			return fmt.Errorf("updating rollout status: %w", err)
		}
		if err := r.repo.RolloutEventCreate(ctx, rollout.GraphVars.RolloutID, true, "Helm install failed"); err != nil {
			return fmt.Errorf("creating rollout event: %w", err)
		}
	case model.RolloutStatusDeployed:
		if err := r.repo.RolloutEventCreate(ctx, rollout.GraphVars.RolloutID, false, "Helm install succeeded"); err != nil {
			return fmt.Errorf("creating rollout event: %w", err)
		}
	default:
		return fmt.Errorf("invalid helm status: %v", helmStatus.RolloutStatus)
	}

	last, err := r.last(ctx, env.Kind, rollout)
	if err != nil {
		return fmt.Errorf("checking if last: %w", err)
	}
	if !last {
		r.log.WithField("name", helmStatus.Name).Info("not last")
		return nil
	}

	status, err := r.repo.RolloutStatus(ctx, rollout.Name)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.WithField("name", helmStatus.Name).Warn("unknown rollout")
			return nil
		}
		return fmt.Errorf("getting rollout status for %q: %w", rollout.Name, err)
	}

	if status == model.RolloutStatusFailed {
		if err := r.repo.RolloutsUpdateStatus(ctx, model.RolloutStatusFailed, rollout.Name, true); err != nil {
			return fmt.Errorf("updating rollout status: %w", err)
		}
		r.log.WithField("name", helmStatus.Name).Info("rollout failed")
		return nil
	}

	return r.repo.TxFunc(ctx, func(repo database.Repo) error {
		if err := repo.FeatureVersionUpdate(ctx, rollout.Name, rollout.Version); err != nil {
			return fmt.Errorf("updating feature version: %w", err)
		}

		if err := repo.RolloutsUpdateStatus(ctx, model.RolloutStatusDeployed, rollout.Name, true); err != nil {
			return fmt.Errorf("updating rollout status: %w", err)
		}

		r.log.WithField("name", helmStatus.Name).Info("rollout succeeded")
		return nil
	})
}

func (r *Receiver) last(ctx context.Context, curr model.EnvironmentKind, rollout *model.Feature) (bool, error) {
	for _, k := range rollout.EnvironmentKinds {
		if k == curr { // don't mind the current environment kind
			continue
		}
		ciEnv, err := r.repo.EnvironmentCI(ctx, curr)
		if err != nil {
			return false, fmt.Errorf("getting CI environment for kind %v: %w", curr.String(), err)
		}
		s, err := r.repo.StatusForFeature(ctx, ciEnv.ID, rollout.Name)
		if err != nil {
			return false, fmt.Errorf("getting status for feature: %w", err)
		}
		if s.Version != rollout.Version {
			return false, nil
		}
	}
	return true, nil
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
		return err
	}

	return r.repo.TxFunc(ctx, func(repo database.Repo) error {
		err = repo.ReleaseStatusDeleteByEnvironmentID(ctx, environmentID)
		if err != nil {
			return err
		}

		for _, rel := range status.Releases {
			err = repo.ReleaseStatusCreateOrUpdate(ctx, environmentID, &rel)
			if err != nil {
				return err
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

	r.log.WithField("nodes", len(status.Nodes)).Debug("received kubernetes nodes")
	return r.repo.KubernetesNodeSync(ctx, environmentID, status)
}
