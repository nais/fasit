package upgrader

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/hashicorp/go-version"
	"github.com/jackc/pgx/v4"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
)

type ClusterUpgrader struct {
	log    logrus.FieldLogger
	repo   database.Repo
	client Upgrader

	// Metrics
	upgradeInProgress metric.Int64Counter
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader Upgrader, meter metric.Meter) *ClusterUpgrader {
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
	var err error
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
			if err := c.upgradeEnvironment(ctx, tenant, env); err != nil {
				return err
			}
			continue
		}
	}
	return nil
}

func (c *ClusterUpgrader) upgradeEnvironment(ctx context.Context, tenant *model.Tenant, env *model.Environment) error {
	log := c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name})
	projectId, err := getProjectId(ctx, c, env.ID)
	if err != nil {
		return err
	}

	clusterUpgrade, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if clusterUpgrade == nil {
		return nil
	}

	runningOperations, err := c.getAndUpdateRunningOperations(ctx, projectId, env, clusterUpgrade)
	if err != nil {
		return err
	}

	log.WithFields(logrus.Fields{"target_version": clusterUpgrade.Version, "status": clusterUpgrade.UpgradeStatus}).Debug("cluster upgrade status")
	switch clusterUpgrade.UpgradeStatus {
	case model.UpgradeStatusCreated:
		// initial state, upgrade master
		log.WithFields(logrus.Fields{"target_version": clusterUpgrade.Version}).Info("starting master upgrade")
		if clusterHas(runningOperations) {
			log.WithFields(logrus.Fields{"target_version": clusterUpgrade.Version}).Debug("has running operations, skipping...")
			return nil
		}

		_, err = c.masterUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectId)
		if err != nil {
			return err
		}

	case model.UpgradeStatusMasterUpgrade:
		// check status on ongoing master upgrade
		status, err := c.masterUpgradeStatus(ctx, env, clusterUpgrade, projectId, tenant.Name)
		if err != nil {
			return err
		}
		if status == nil {
			// upgrade not done
			return nil
		}
		log.WithFields(logrus.Fields{"target_version": status.Version}).Info("master upgrade done")

	case model.UpgradeStatusNodeUpgrade:
		if clusterHas(runningOperations) {
			log.WithFields(logrus.Fields{"target_version": clusterUpgrade.Version}).Debug("has running operations, skipping...")
			return nil
		}

		// check status on node upgrade
		if done, err := c.nodeUpgradeStatus(ctx, env, clusterUpgrade, projectId); !done {
			if err != nil {
				return err
			}

			// update
			un, err := c.upgradeNodes(ctx, env, clusterUpgrade, projectId, tenant.Name)
			if err != nil {
				return err
			}
			if un != nil {
				// we started a upgrade
				log.WithField("target_version", un.Version).Info("nodepool upgrade started")
				return nil
			}
		}

		// node upgrade done, update status
		log.Debug("node upgrade done")
		upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusDONE, clusterUpgrade.Version)
		if err != nil {
			return err
		}
		c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "nodePools")...))
		log.WithField("target_version", upgradeStatus.Version).Info("nodepool upgrade done")
	}
	return nil
}

func clusterHas(runningOperations []*containerpb.Operation) bool {
	for _, op := range runningOperations {
		if op.Status == containerpb.Operation_RUNNING {
			return true
		}
	}
	return false
}

func (c *ClusterUpgrader) getAndUpdateRunningOperations(ctx context.Context, projectId string, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus) ([]*containerpb.Operation, error) {
	// checks if there are any running operations for the environment
	runningOperations, err := c.client.GetRunningOperations(ctx, projectId, env)
	if err != nil {
		return nil, err
	}

	// checks type of operation. if different from UPGRADE_NODES or UPGRADE_MASTER, then skip, else update operation in db
	for _, op := range runningOperations {
		if op.OperationType != containerpb.Operation_UPGRADE_NODES && op.OperationType != containerpb.Operation_UPGRADE_MASTER {
			return nil, nil
		}

		_, err := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
		if err != nil {
			return nil, err
		}

		_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatus(clusterUpgrade.UpgradeStatus), clusterUpgrade.Version)
		if err != nil {
			return nil, err
		}
	}
	return runningOperations, nil
}

func (c *ClusterUpgrader) upgradeNodes(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectId, tenantName string) (*model.ClusterUpgradeStatus, error) {
	nodePools, err := c.client.GetNodePools(ctx, projectId, env)
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

		op, err := c.client.UpgradeNodePool(ctx, projectId, env, np.Name, clusterUpgrade.Version)
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

func (c *ClusterUpgrader) nodeUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectId string) (bool, error) {
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
	op, err := c.client.UpgradeMaster(ctx, projectId, env, upgrade.Version)
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
	nodepools, err := c.client.GetNodePools(ctx, projectId, env)
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

func (c *ClusterUpgrader) IsNewerPatchRelease(current, new string) bool {
	v1, err := version.NewVersion(current)
	if err != nil {
		c.log.Fatalf("Error parsing version1: %s", err)
	}

	v2, err := version.NewVersion(new)
	if err != nil {
		c.log.Fatalf("Error parsing version2: %s", err)
	}

	// Split versions to extract major.minor.patch
	v1Segments := strings.Split(v1.String(), "-")[0]
	v2Segments := strings.Split(v2.String(), "-")[0]

	v1Parts := strings.Split(v1Segments, ".")
	v2Parts := strings.Split(v2Segments, ".")

	if len(v1Parts) < 3 || len(v2Parts) < 3 {
		c.log.Fatalf("Invalid version format, must include major.minor.patch")
	}

	// Compare major and minor versions
	if v1Parts[0] == v2Parts[0] && v1Parts[1] == v2Parts[1] {
		return v2.GreaterThan(v1)
	}

	return false
}
