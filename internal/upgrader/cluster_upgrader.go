package upgrader

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

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

	// Check if upgrade is stuck (> 24 hours in same state)
	if c.isUpgradeStuck(clusterUpgrade) {
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"status":         clusterUpgrade.UpgradeStatus,
			"last_modified":  clusterUpgrade.LastModified,
		}).Warn("cluster upgrade stuck, marking as failed")

		_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusFAILED, clusterUpgrade.Version)
		if err != nil {
			log.WithError(err).Error("failed to mark stuck upgrade as failed")
			return err
		}

		// Send Slack notification about stuck upgrade
		c.notifyStuckUpgrade(tenant.Name, env.Name, clusterUpgrade)
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
			c.logNonCriticalError(err, "get_upgrade_mentions", logrus.Fields{
				"tenant":      tenant.Name,
				"environment": env.Name,
			})
			mentions = "" // Use empty mentions as fallback
		}

		msg := c.slack.GetClusterUpgradeNotificationMessageOptions(tenant.Name, env.Name, clusterUpgrade.Version, "control plane", mentions)

		channelID, timestamp, err := c.slack.PostMessage(c.slackChannel, msg)
		if err != nil {
			c.logNonCriticalError(err, "slack_post_message", logrus.Fields{
				"tenant":      tenant.Name,
				"environment": env.Name,
				"version":     clusterUpgrade.Version,
			})
		}
		_, err = c.repo.SetClusterUpgradesSlackMessage(ctx, clusterUpgrade.ID, timestamp, channelID)
		if err != nil {
			c.logNonCriticalError(err, "set_slack_message_metadata", logrus.Fields{
				"tenant":             tenant.Name,
				"environment":        env.Name,
				"cluster_upgrade_id": clusterUpgrade.ID,
			})
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
			c.logNonCriticalError(err, "slack_post_comment", logrus.Fields{
				"tenant":      tenant.Name,
				"environment": env.Name,
				"operation":   "upgrade_complete",
			})
		}

		err = c.slack.AddReaction(upgradeStatus.SlackChannelID, upgradeStatus.SlackMessageTimestamp, "white_check_mark")
		if err != nil {
			c.logNonCriticalError(err, "slack_add_reaction", logrus.Fields{
				"tenant":      tenant.Name,
				"environment": env.Name,
				"reaction":    "white_check_mark",
			})
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
	var runningOperations []*containerpb.Operation
	err := c.retryWithBackoff(ctx, "get_running_operations", func() error {
		var retryErr error
		runningOperations, retryErr = c.client.GetRunningOperations(ctx, projectID, env)
		return retryErr
	})
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
	var nodePools []*containerpb.NodePool
	err := c.retryWithBackoff(ctx, "get_node_pools", func() error {
		var retryErr error
		nodePools, retryErr = c.client.GetNodePools(ctx, projectID, env)
		return retryErr
	})
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

		var op *containerpb.Operation
		// Retry the GKE nodepool upgrade API call with exponential backoff
		retryErr := c.retryWithBackoff(ctx, "upgrade_nodepool", func() error {
			var err error
			op, err = c.client.UpgradeNodePool(ctx, projectID, env, np.Name, clusterUpgrade.Version)
			return err
		})
		if retryErr != nil {
			return nil, retryErr
		}

		msg := c.slack.GetClusterUpgradeNotificationMessageOptions(tenantName, env.Name, clusterUpgrade.Version, np.Name, "")

		err = c.slack.PostComment(c.slackChannel, clusterUpgrade.SlackMessageTimestamp, msg)
		if err != nil {
			c.logNonCriticalError(err, "slack_post_comment", logrus.Fields{
				"tenant":      tenantName,
				"environment": env.Name,
				"nodepool":    np.Name,
				"operation":   "nodepool_upgrade_start",
			})
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
	var op *containerpb.Operation
	
	// Retry the GKE API call with exponential backoff
	err := c.retryWithBackoff(ctx, "upgrade_master", func() error {
		var retryErr error
		op, retryErr = c.client.UpgradeMaster(ctx, projectID, env, upgrade.Version)
		return retryErr
	})
	
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
	var nodepools []*containerpb.NodePool
	err := c.retryWithBackoff(ctx, "get_node_pools_status", func() error {
		var retryErr error
		nodepools, retryErr = c.client.GetNodePools(ctx, projectID, env)
		return retryErr
	})
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
	var op *containerpb.Operation
	err := c.retryWithBackoff(ctx, "get_operation_status", func() error {
		var retryErr error
		op, retryErr = c.client.GetOperation(ctx, projectID, operationName)
		return retryErr
	})
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

// isRetriableError checks if an error should be retried
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a GKE API error
	if apiErr, ok := err.(*apierror.APIError); ok {
		// Retriable errors: rate limits, temporary unavailable, etc.
		switch apiErr.GRPCStatus().Code() {
		case codes.Unavailable, codes.DeadlineExceeded, codes.Aborted:
			return true
		case codes.InvalidArgument, codes.NotFound, codes.PermissionDenied:
			return false // Permanent errors
		}
	}

	// Database connection errors are typically retriable
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	// Default to non-retriable for safety
	return false
}

// logNonCriticalError logs errors that don't stop the operation but should be noted
func (c *ClusterUpgrader) logNonCriticalError(err error, operation string, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["operation"] = operation
	fields["retriable"] = isRetriableError(err)

	c.log.WithFields(fields).WithError(err).Warn("non-critical operation failed")
}

// retryWithBackoff executes a function with exponential backoff for retriable errors
func (c *ClusterUpgrader) retryWithBackoff(ctx context.Context, operation string, fn func() error) error {
	const maxRetries = 3
	const baseDelay = 1 * time.Second
	const maxDelay = 30 * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 0 {
				c.log.WithFields(logrus.Fields{
					"operation": operation,
					"attempt":   attempt + 1,
					"success":   true,
				}).Info("operation succeeded after retry")
			}
			return nil
		}

		lastErr = err
		
		// Don't retry non-retriable errors
		if !isRetriableError(err) {
			c.log.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt + 1,
				"retriable": false,
			}).WithError(err).Debug("non-retriable error, not retrying")
			return err
		}

		// Don't retry on last attempt
		if attempt == maxRetries {
			break
		}

		// Calculate delay with exponential backoff and jitter
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(attempt)))
		if delay > maxDelay {
			delay = maxDelay
		}

		c.log.WithFields(logrus.Fields{
			"operation":    operation,
			"attempt":      attempt + 1,
			"next_attempt": attempt + 2,
			"delay":        delay.String(),
		}).WithError(err).Warn("retriable error, retrying after delay")

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	c.log.WithFields(logrus.Fields{
		"operation":    operation,
		"max_attempts": maxRetries + 1,
		"final_error":  lastErr,
	}).Error("operation failed after all retry attempts")

	return lastErr
}

// isUpgradeStuck checks if a cluster upgrade has been in the same state for more than 24 hours
func (c *ClusterUpgrader) isUpgradeStuck(clusterUpgrade *model.ClusterUpgradeStatus) bool {
	// Don't consider DONE or FAILED as stuck
	if clusterUpgrade.UpgradeStatus == model.UpgradeStatusDone || clusterUpgrade.UpgradeStatus == model.UpgradeStatusFailed {
		return false
	}

	// Check if upgrade has been running for more than 24 hours
	stuckThreshold := 24 * time.Hour
	return time.Since(clusterUpgrade.LastModified) > stuckThreshold
}

// notifyStuckUpgrade sends a Slack notification about a stuck upgrade
func (c *ClusterUpgrader) notifyStuckUpgrade(tenantName, envName string, clusterUpgrade *model.ClusterUpgradeStatus) {
	// Build Slack message using the same pattern as other notifications
	stuckMsg := c.slack.GetClusterUpgradeStuckNotificationMessageOptions(
		tenantName,
		envName,
		clusterUpgrade.Version,
		string(clusterUpgrade.UpgradeStatus),
		clusterUpgrade.LastModified.Format("2006-01-02 15:04:05"))

	if clusterUpgrade.SlackMessageTimestamp != "" {
		// Reply to existing upgrade message
		err := c.slack.PostComment(c.slackChannel, clusterUpgrade.SlackMessageTimestamp, stuckMsg)
		if err != nil {
			c.logNonCriticalError(err, "slack_post_stuck_comment", logrus.Fields{
				"tenant":      tenantName,
				"environment": envName,
				"operation":   "stuck_upgrade_notification",
			})
		}
	} else {
		// Post new message
		_, _, err := c.slack.PostMessage(c.slackChannel, stuckMsg)
		if err != nil {
			c.logNonCriticalError(err, "slack_post_stuck_message", logrus.Fields{
				"tenant":      tenantName,
				"environment": envName,
				"operation":   "stuck_upgrade_notification",
			})
		}
	}
}
