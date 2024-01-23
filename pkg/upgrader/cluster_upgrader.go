package upgrader

import (
	"context"
	"encoding/json"
	"errors"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/model"

	"github.com/sirupsen/logrus"
)

type ClusterUpgrader struct {
	log    logrus.FieldLogger
	repo   database.Repo
	client graph.Upgrader
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader graph.Upgrader) *ClusterUpgrader {
	return &ClusterUpgrader{
		log:    log,
		repo:   repo,
		client: upgrader,
	}
}

func (c *ClusterUpgrader) Run(ctx context.Context) error {
	c.log.Info("Running cluster upgrader")
	tenants, err := c.repo.TenantsGet(ctx)
	if err != nil {
		return err
	}
	for _, tenant := range tenants {
		envs, err := c.repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			return err
		}
		for _, env := range envs {
			projectId, err := getProjectId(ctx, c, env.ID)
			if err != nil {
				return err
			}

			c.log.Debugf("checking for cluster upgrade %s/%s", tenant.Name, env.Name)

			clusterUpgrade, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}

			// nothing to do
			if clusterUpgrade == nil {
				continue
			}

			// check status on ongoing master upgrade
			if clusterUpgrade.UpgradeStatus == model.UpgradeStatusMasterUpgrade {
				co, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
				if err != nil {
					return err
				}

				op, err := c.client.GetOperation(ctx, projectId, co.ID)
				if err != nil {
					return err
				}

				_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
				if err != nil {
					return err
				}

				if op.Status == containerpb.Operation_DONE {
					upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgrade.Version)
					if err != nil {
						return err
					}
					c.log.Infof("api server upgrade (%s) done - %s/%s", tenant.Name, env.Name, upgradeStatus.Version)
					continue
				}
			}

			// upgrade master
			if clusterUpgrade.UpgradeStatus == model.UpgradeStatusCreated {
				c.log.Debugf("cluster upgrade created - %q/%q", tenant.Name, env.Name)
				ops, err := c.client.GetRunningOperations(ctx, projectId, env.Name)
				if err != nil {
					return err
				}

				if len(ops) > 0 {
					c.log.Debugf("found %d running operations for tenant %s, env %s", len(ops), tenant.Name, env.Name)
					continue
				}

				op, err := c.client.UpgradeMaster(ctx, projectId, env.Name, clusterUpgrade.Version)
				if err != nil {
					return err
				}

				_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
				if err != nil {
					return err
				}

				upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusMASTERUPGRADE, clusterUpgrade.Version)
				if err != nil {
					return err
				}
				c.log.Infof("api server upgrade (%s) started - %s/%s", tenant.Name, env.Name, upgradeStatus.Version)
			}

			// upgrade nodes
			// TODO: check if upgrade is done

		}

	}
	return nil
}

func getProjectId(ctx context.Context, c *ClusterUpgrader, environmentId uuid.UUID) (string, error) {
	projectId, err := c.repo.EnvironmentValueGet(ctx, environmentId, "project_id", false)
	if err != nil {
		return "", err
	}

	id := ""
	if err := json.Unmarshal(projectId.Value, &id); err != nil {
		return "", err
	}

	return id, nil
}
