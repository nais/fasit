package upgrader

import (
	"cloud.google.com/go/container/apiv1/containerpb"
	"context"
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2/apierror"
	version "github.com/hashicorp/go-version"
	"github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/model"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"

	"github.com/sirupsen/logrus"
)

type ClusterUpgrader struct {
	log    logrus.FieldLogger
	repo   database.Repo
	client graph.Upgrader

	// Metrics
	upgradeInProgress metric.Int64Counter
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader graph.Upgrader, meter metric.Meter) *ClusterUpgrader {
	counter, err := meter.Int64Counter("upgrade_in_progress", metric.WithDescription("Upgrade in progress"))
	if err != nil {
		log.Fatal(err)
	}

	return &ClusterUpgrader{
		log:               log,
		repo:              repo,
		client:            upgrader,
		upgradeInProgress: counter,
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
	ENVS:
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
				continue ENVS
			}

			// checks if there are any running operations for the environment
			runningOperations, err := c.client.GetRunningOperations(ctx, projectId, env.Name)
			if err != nil {
				return err
			}

			// checks type of operation. if different from UPGRADE_NODES or UPGRADE_MASTER, then skip, else update operation in db
			if len(runningOperations) > 0 {
				c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("found %d running operation(s) for environment", len(runningOperations))
				for _, op := range runningOperations {
					if op.OperationType != containerpb.Operation_UPGRADE_NODES && op.OperationType != containerpb.Operation_UPGRADE_MASTER {
						continue ENVS
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

			c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Debug("upgrade cluster")
			switch clusterUpgrade.UpgradeStatus {
			case model.UpgradeStatusMasterUpgrade:
				// check status on ongoing master upgrade
				_, err := c.masterUpgradeStatus(ctx, env, clusterUpgrade, projectId, tenant.Name)
				if err != nil {
					return err
				}
				continue ENVS
			case model.UpgradeStatusNodeUpgrade:
				// check if there are any running operations for the environment
				for _, op := range runningOperations {
					if op.Status == containerpb.Operation_RUNNING {
						c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("found %s/%s running for environment, skipping", op.Name, op.OperationType)
						continue ENVS
					}
				}

				// check status on node upgrade and update
				done, err := c.nodeUpgradeStatus(ctx, env, clusterUpgrade, projectId, tenant.Name)
				if err != nil {
					return err
				}

				if !done {
					us, err := c.upgradeNodes(ctx, env, clusterUpgrade, projectId, tenant.Name)
					if err != nil {
						return err
					}

					if us != nil {
						continue ENVS
					}
				}

				upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusDONE, clusterUpgrade.Version)
				if err != nil {
					return err
				}
				c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "nodePools")...))
				c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("nodepool upgrade to (%s) done", upgradeStatus.Version)

			case model.UpgradeStatusCreated:
				// initial state, upgrade master
				c.log.Debugf("cluster upgrade created - %s/%s", tenant.Name, env.Name)
				ops, err := c.client.GetRunningOperations(ctx, projectId, env.Name)
				if err != nil {
					return err
				}

				if len(ops) > 0 {
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("found %d running operations for environment, skipping", len(ops))
					continue ENVS
				}

				_, err = c.masterUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectId)
				if err != nil {
					return err
				}
				continue ENVS
			}
		}
	}
	return nil
}

func (c *ClusterUpgrader) upgradeNodes(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectId, tenantName string) (*model.ClusterUpgradeStatus, error) {
	nodePools, err := c.client.GetNodePools(ctx, projectId, env.Name)
	if err != nil {
		return nil, err
	}

	clusterUpgraderVersionObj, err := version.NewVersion(clusterUpgrade.Version)
	if err != nil {
		return nil, err
	}

	for _, np := range nodePools {
		npVersionObj, err := version.NewVersion(np.Version)
		if err != nil {
			return nil, err
		}

		if npVersionObj.GreaterThanOrEqual(clusterUpgraderVersionObj) {
			continue
		}

		op, err := c.client.UpgradeNodePool(ctx, projectId, env.Name, np.Name, clusterUpgrade.Version)
		if err != nil {
			return nil, err
		}

		c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "nodePools")...))
		_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
		if err != nil {
			return nil, err
		}

		us, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatus(clusterUpgrade.UpgradeStatus), clusterUpgrade.Version)
		if err != nil {
			return nil, err
		}
		c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("started upgrade of nodepool %s to %s", np.Name, clusterUpgrade.Version)
		return us, nil
	}
	return nil, nil
}

func (c *ClusterUpgrader) nodeUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectId, tenantName string) (bool, error) {
	rop, err := c.repo.GetRunningClusterOperation(ctx, env.TenantID, env.ID)
	if err != nil {
		return false, err
	}

	if rop != nil {
		_, err := c.getAndUpdateOperation(ctx, projectId, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
		if err != nil {
			return false, err
		}
	}

	done, err := c.clusterNodePoolsCompleted(ctx, projectId, env, clusterUpgrade)
	if err != nil {
		return done, err
	}

	return done, nil
}

func (c *ClusterUpgrader) masterUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectId, tenantName string) (*model.ClusterUpgradeStatus, error) {
	rop, err := c.repo.GetRunningClusterOperation(ctx, env.TenantID, env.ID)
	if err != nil {
		return nil, err
	}

	op, err := c.getAndUpdateOperation(ctx, projectId, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
	if err != nil {
		return nil, err
	}

	var upgradeStatus *model.ClusterUpgradeStatus
	if op.Status == containerpb.Operation_DONE {
		c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(
			setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "master")...),
		)
		upgradeStatus, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusNODEUPGRADE, clusterUpgrade.Version)
		if err != nil {
			return nil, err
		}
		c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("api server upgrade to %s done", upgradeStatus.Version)
	}
	return upgradeStatus, nil
}

func (c *ClusterUpgrader) masterUpgrade(ctx context.Context, env *model.Environment, upgrade *model.ClusterUpgradeStatus, tenantName, projectId string) (*model.ClusterUpgradeStatus, error) {
	op, err := c.client.UpgradeMaster(ctx, projectId, env.Name, upgrade.Version)
	if err != nil {
		if e, ok := err.(*apierror.APIError); ok {
			if e.GRPCStatus().Code() == codes.InvalidArgument {
				c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("invalid argument: %s", e.GRPCStatus().Message())
				_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusFAILED, upgrade.Version)
				if err != nil {
					return nil, err
				}
			}
		}
		return nil, err
	}

	c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(
		setMetricsAttrs(env.Name, tenantName, upgrade.Version, "master")...))

	_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, upgrade.ID, op)
	if err != nil {
		return nil, err
	}

	upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusMASTERUPGRADE, upgrade.Version)
	if err != nil {
		return upgradeStatus, err
	}
	c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("api server upgrade to %s started", upgradeStatus.Version)
	return upgradeStatus, nil
}

func setMetricsAttrs(envName, tenantName, version, target string) []attribute.KeyValue {
	metricAttrs := []attribute.KeyValue{
		attribute.String("environment", envName),
		attribute.String("tenant", tenantName),
		attribute.String("version", version),
		attribute.String("target", target),
	}
	return metricAttrs
}

func (c *ClusterUpgrader) clusterNodePoolsCompleted(ctx context.Context, projectId string, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus) (bool, error) {
	nodepools, err := c.client.GetNodePools(ctx, projectId, env.Name)
	if err != nil {
		return false, err
	}

	clusterUpgraderVersionObj, err := version.NewVersion(clusterUpgrade.Version)
	if err != nil {
		return false, err
	}

	done := true
	for _, np := range nodepools {
		npVersionObj, err := version.NewVersion(np.Version)
		if err != nil {
			return false, err
		}
		if !npVersionObj.Equal(clusterUpgraderVersionObj) {
			done = false
		}
	}
	return done, nil
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
