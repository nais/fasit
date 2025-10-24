package upgrader

import (
	"context"
	"encoding/json"
	"errors"

	"cloud.google.com/go/container/apiv1/containerpb"
	"github.com/google/uuid"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/hashicorp/go-version"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/slack"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc/codes"
)

type ClusterUpgrader struct {
	log          logrus.FieldLogger
	repo         database.Repo
	client       Upgrader
	slack        slack.SlackClient
	slackChannel string

	// Metrics
	upgradeInProgress metric.Int64Counter
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader Upgrader, meter metric.Meter, slack slack.SlackClient, slackChannel string) *ClusterUpgrader {
	counter, err := meter.Int64Counter("upgrade_in_progress", metric.WithDescription("Upgrade in progress"))
	if err != nil {
		log.Fatal(err)
	}

	return &ClusterUpgrader{
		log:               log,
		repo:              repo,
		client:            upgrader,
		upgradeInProgress: counter,
		slack:             slack,
		slackChannel:      slackChannel,
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
	projectID, err := getProjectID(ctx, c, env.ID)
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

	runningOperations, err := c.getAndUpdateRunningOperations(ctx, projectID, env, clusterUpgrade)
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

		_, err = c.masterUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectID)
		if err != nil {
			return err
		}

		mentions, err := getUpgradeMentions(ctx, c, env.ID)
		if err != nil {
			c.log.WithField("error", err).Error("failed to get upgrade mentions")
		}

		msg := c.slack.GetClusterUpgradeNotificationMessageOptions(tenant.Name, env.Name, clusterUpgrade.Version, "control plane", mentions)

		channelID, timestamp, err := c.slack.PostMessage(c.slackChannel, msg)
		if err != nil {
			c.log.WithFields(logrus.Fields{"error": err}).Error("failed to post message to slack")
		}
		_, err = c.repo.SetClusterUpgradesSlackMessage(ctx, clusterUpgrade.ID, timestamp, channelID)
		if err != nil {
			c.log.WithField("error", err).Error("failed to set slack message")
		}

	case model.UpgradeStatusMasterUpgrade:
		// check status on ongoing master upgrade
		status, err := c.masterUpgradeStatus(ctx, env, clusterUpgrade, projectID, tenant.Name)
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
		if done, err := c.nodeUpgradeStatus(ctx, env, clusterUpgrade, projectID); !done {
			if err != nil {
				return err
			}

			// update
			un, err := c.upgradeNodes(ctx, env, clusterUpgrade, projectID, tenant.Name)
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

		msg := c.slack.GetClusterUpgradeDoneNotificationMessageOptions(tenant.Name, env.Name)

		err = c.slack.PostComment(c.slackChannel, clusterUpgrade.SlackMessageTimestamp, msg)
		if err != nil {
			c.log.WithFields(logrus.Fields{"error": err}).Error("failed to post comment to slack")
		}

		err = c.slack.AddReaction(upgradeStatus.SlackChannelID, upgradeStatus.SlackMessageTimestamp, "white_check_mark")
		if err != nil {
			c.log.WithFields(logrus.Fields{"error": err}).Error("failed to add reaction to slack")
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

func (c *ClusterUpgrader) getAndUpdateRunningOperations(ctx context.Context, projectID string, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus) ([]*containerpb.Operation, error) {
	// checks if there are any running operations for the environment
	runningOperations, err := c.client.GetRunningOperations(ctx, projectID, env)
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

func (c *ClusterUpgrader) upgradeNodes(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectID, tenantName string) (*model.ClusterUpgradeStatus, error) {
	nodePools, err := c.client.GetNodePools(ctx, projectID, env)
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

		op, err := c.client.UpgradeNodePool(ctx, projectID, env, np.Name, clusterUpgrade.Version)
		if err != nil {
			return nil, err
		}

		msg := c.slack.GetClusterUpgradeNotificationMessageOptions(tenantName, env.Name, clusterUpgrade.Version, np.Name, "")

		err = c.slack.PostComment(c.slackChannel, clusterUpgrade.SlackMessageTimestamp, msg)
		if err != nil {
			c.log.WithFields(logrus.Fields{"error": err}).Error("failed to post comment to slack")
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

func (c *ClusterUpgrader) nodeUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectID string) (bool, error) {
	rop, err := c.repo.GetRunningClusterOperation(ctx, env.TenantID, env.ID)
	if err != nil {
		return false, err
	}

	if rop != nil {
		_, err := c.getAndUpdateOperation(ctx, projectID, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
		if err != nil {
			return false, err
		}
	}

	done, err := c.clusterNodePoolsCompleted(ctx, projectID, env, clusterUpgrade)
	if err != nil {
		return done, err
	}

	return done, nil
}

func (c *ClusterUpgrader) masterUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectID, tenantName string) (*model.ClusterUpgradeStatus, error) {
	rop, err := c.repo.GetRunningClusterOperation(ctx, env.TenantID, env.ID)
	if err != nil {
		return nil, err
	}

	op, err := c.getAndUpdateOperation(ctx, projectID, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
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

func (c *ClusterUpgrader) masterUpgrade(ctx context.Context, env *model.Environment, upgrade *model.ClusterUpgradeStatus, tenantName, projectID string) (*model.ClusterUpgradeStatus, error) {
	op, err := c.client.UpgradeMaster(ctx, projectID, env, upgrade.Version)
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

func (c *ClusterUpgrader) clusterNodePoolsCompleted(ctx context.Context, projectID string, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus) (bool, error) {
	nodepools, err := c.client.GetNodePools(ctx, projectID, env)
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

func (c *ClusterUpgrader) getAndUpdateOperation(ctx context.Context, projectID string, tenantID, envID, clusterUpgradeID uuid.UUID, operationName string) (*containerpb.Operation, error) {
	op, err := c.client.GetOperation(ctx, projectID, operationName)
	if err != nil {
		return nil, err
	}

	_, err = c.repo.CreateOrUpdateClusterOperation(ctx, tenantID, envID, clusterUpgradeID, op)
	if err != nil {
		return nil, err
	}

	return op, nil
}

func getProjectID(ctx context.Context, c *ClusterUpgrader, environmentID uuid.UUID) (string, error) {
	projectID, err := c.repo.EnvironmentValueGet(ctx, environmentID, "project_id", false)
	if err != nil {
		return "", err
	}

	id := ""
	if err := json.Unmarshal(projectID.Value, &id); err != nil {
		return "", err
	}

	return id, nil
}

func getUpgradeMentions(ctx context.Context, c *ClusterUpgrader, environmentID uuid.UUID) (string, error) {
	notifications, err := c.repo.EnvironmentValueGet(ctx, environmentID, "slack_upgrade_mentions", false)
	if err != nil {
		return "", err
	}

	notificationsString := ""
	if err := json.Unmarshal(notifications.Value, &notificationsString); err != nil {
		return "", err
	}

	return notificationsString, nil
}
