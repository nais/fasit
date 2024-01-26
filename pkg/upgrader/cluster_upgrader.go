package upgrader

import (
	"context"
	"encoding/json"
	"errors"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	version "github.com/hashicorp/go-version"
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
	c.log.Debug("running scheduled cluster upgrader")
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

			clusterUpgrade, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return err
			}

			// nothing to do
			if clusterUpgrade == nil {
				continue
			}

			// checks if there are any running operations for the environment
			runningOperations, err := c.client.GetRunningOperations(ctx, projectId, env.Name)
			if err != nil {
				return err
			}

			// checks type of operation. if different from UPGRADE_NODES or UPGRADE_MASTER, then skip, else update operation in db
			skipEnv := false
			if len(runningOperations) > 0 {
				c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("found %d running operation(s) for environment", len(runningOperations))
				for _, op := range runningOperations {
					if op.OperationType != containerpb.Operation_UPGRADE_NODES && op.OperationType != containerpb.Operation_UPGRADE_MASTER {
						skipEnv = true
					}

					_, err := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
					if err != nil {
						return err
					}

					_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatus(clusterUpgrade.UpgradeStatus), clusterUpgrade.Version)
					if err != nil {
						return err
					}
				}
			}
			if skipEnv {
				continue
			}

			c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Debug("upgrade cluster")

			// get cluster operation for cluster upgrade from db
			/*co, err := c.repo.ClusterOperationsGetByUpgradeIDAndStatus(ctx, clusterUpgrade.ID, containerpb.Operation_RUNNING.String())
			if err != nil {
				return err
			}

			/*
				if co == nil && clusterUpgrade.UpgradeStatus != model.UpgradeStatusCreated {
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Warning("no running cluster operation found, forcing next step")
					var status gensql.ClusterUpgradesStatus
					if clusterUpgrade.UpgradeStatus == model.UpgradeStatusMasterUpgrade {
						status = gensql.ClusterUpgradesStatusNODEUPGRADE
					} else if clusterUpgrade.UpgradeStatus == model.UpgradeStatusNodeUpgrade {
						status = gensql.ClusterUpgradesStatusDONE
					}
					clusterUpgrade, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, status, clusterUpgrade.Version)
					if err != nil {
						return err
					}
					continue
				}*/

			// check status on ongoing master upgrade

			if clusterUpgrade.UpgradeStatus == model.UpgradeStatusMasterUpgrade {
				rop, err := c.repo.GetRunningClusterOperation(ctx, env.TenantID, env.ID)
				if err != nil {
					return err
				}

				op, err := c.getAndUpdateOperation(ctx, projectId, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
				if err != nil {
					return err
				}

				if op.Status == containerpb.Operation_DONE {
					upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgrade.Version)
					if err != nil {
						return err
					}
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("api server upgrade to %s done", upgradeStatus.Version)
					continue
				}
			}

			// check status on ongoing node upgrade
			if clusterUpgrade.UpgradeStatus == model.UpgradeStatusNodeUpgrade {
				rop, err := c.repo.GetRunningClusterOperation(ctx, env.TenantID, env.ID)
				if err != nil {
					return err
				}

				if rop != nil {
					op, err := c.getAndUpdateOperation(ctx, projectId, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
					if err != nil {
						return err
					}

					if op.Status == containerpb.Operation_DONE && op.OperationType == containerpb.Operation_UPGRADE_NODES {
						done := true
						nodepools, err := c.client.GetNodePools(ctx, projectId, env.Name)
						if err != nil {
							return err
						}

						clusterUpgraderVersionObj, err := version.NewVersion(clusterUpgrade.Version)
						if err != nil {
							return err
						}

						for _, np := range nodepools {
							npVersionObj, err := version.NewVersion(np.Version)
							if err != nil {
								return err
							}
							if npVersionObj.LessThan(clusterUpgraderVersionObj) {
								done = false
							}
						}
						if done {
							upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusDONE, clusterUpgrade.Version)
							if err != nil {
								return err
							}
							c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("nodepool upgrade to (%s) done", upgradeStatus.Version)
							continue
						}
					} else if op.Status == containerpb.Operation_RUNNING {
						_, err := c.getAndUpdateOperation(ctx, projectId, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
						if err != nil {
							return err
						}
						c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("nodepool upgrade to (%s) running", clusterUpgrade.Version)
						continue
					}
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
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("found %d running operations for environment", len(ops))
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
				c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("api server upgrade to %s started", upgradeStatus.Version)
			}

			// upgrade nodes
			if clusterUpgrade.UpgradeStatus == model.UpgradeStatusNodeUpgrade {
				nodeUpgradeRunning := false
				for _, op := range runningOperations {
					if op.OperationType == containerpb.Operation_UPGRADE_NODES {
						nodeUpgradeRunning = true
					}
				}
				if nodeUpgradeRunning {
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("nodepool upgrade to (%s) running", clusterUpgrade.Version)
					continue
				}
				nodePools, err := c.client.GetNodePools(ctx, projectId, env.Name)
				if err != nil {
					return err
				}

				noUpgradeNeeded := true

				clusterUpgraderVersionObj, err := version.NewVersion(clusterUpgrade.Version)
				if err != nil {
					return err
				}

				for _, np := range nodePools {
					npVersionObj, err := version.NewVersion(np.Version)
					if err != nil {
						return err
					}

					if npVersionObj.LessThan(clusterUpgraderVersionObj) {
						noUpgradeNeeded = false
						op, err := c.client.UpgradeNodePool(ctx, projectId, env.Name, np.Name, clusterUpgrade.Version)
						if err != nil {
							return err
						}

						_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
						if err != nil {
							return err
						}

						_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatus(clusterUpgrade.UpgradeStatus), clusterUpgrade.Version)
						if err != nil {
							return err
						}

						c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("started upgrade of nodepool %s to %s", np.Name, clusterUpgrade.Version)
						break
					}

				}
				if noUpgradeNeeded {
					upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusDONE, clusterUpgrade.Version)
					if err != nil {
						return err
					}
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("nodepool upgrade to (%s) done", upgradeStatus.Version)
				}
			}

		}

	}
	return nil
}

func (c *ClusterUpgrader) getAndUpdateOperation(ctx context.Context, projectId string, tenantId, envId, clusterUpgradeId uuid.UUID, operationName string) (*containerpb.Operation, error) {
	op, err := c.client.GetOperation(ctx, projectId, operationName)
	if err != nil {
		return nil, err
	}

	_, err = c.repo.CreateOrUpdateClusterOperation(ctx, tenantId, envId, clusterUpgradeId, op)
	if err != nil {
		return nil, err
	}

	return op, nil
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
