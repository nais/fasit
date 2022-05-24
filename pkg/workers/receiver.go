package workers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

type ReceiverClient interface {
	Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error
}

type ReceiverStore interface {
	EnvironmentIDByNames(ctx context.Context, tenantName string, environmentName string) (uuid.UUID, error)
	StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Helm) error
	ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error
	TenantGetByName(ctx context.Context, name string) (*model.Tenant, error)
	TenantCreate(ctx context.Context, t *model.TenantCreate) (*model.Tenant, error)
	EnvironmentCreate(ctx context.Context, t *model.EnvironmentCreate) (*model.Environment, error)
	HealthStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Health) error
	KubernetesNodeSync(ctx context.Context, envID uuid.UUID, kn *message.KubernetesNodes) error
}

type Receiver struct {
	manager ReceiverClient
	repo    ReceiverStore
	log     *logrus.Entry
}

func NewReceiver(mgr ReceiverClient, repo ReceiverStore, log *logrus.Entry) *Receiver {
	receiver := &Receiver{manager: mgr, repo: repo, log: log}
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
	err = r.repo.StatusCreateOrUpdate(ctx, environmentID, helmStatus)
	if err != nil {
		return err
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
		return err
	}

	for _, rel := range status.Releases {
		err = r.repo.ReleaseStatusCreateOrUpdate(ctx, environmentID, &rel)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Receiver) healthStatus(ctx context.Context, msg message.Status) error {
	status := &message.Health{}
	err := json.Unmarshal(msg.Data, status)
	if err != nil {
		r.log.WithError(err).Errorf("invalid json")
		return nil
	}
	tenant, err := r.repo.TenantGetByName(ctx, msg.Tenant)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		tenant, err = r.repo.TenantCreate(ctx, &model.TenantCreate{
			Name: msg.Tenant,
		})
		if err != nil {
			return err
		}
	}

	environmentID, err := r.repo.EnvironmentIDByNames(ctx, tenant.Name, msg.Environment)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		env, err := r.repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
			Name:     msg.Environment,
			TenantID: tenant.ID,
			Kind:     status.Kind,
		})
		if err != nil {
			return err
		}
		environmentID = env.ID
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

	r.log.WithField("nodes", len(status.Nodes)).Info("received kubernetes nodes")
	return r.repo.KubernetesNodeSync(ctx, environmentID, status)
}
