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
	upgradeStarted    metric.Int64Counter
	upgradeCompleted  metric.Int64Counter
	upgradeFailed     metric.Int64Counter
	upgradeStuck      metric.Int64Counter
	gkeApiCalls       metric.Int64Counter
	gkeApiErrors      metric.Int64Counter
	retryAttempts     metric.Int64Counter
	upgradeDuration   metric.Float64Histogram
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader Upgrader, meter metric.Meter, slack slack.SlackClient, slackChannel string) *ClusterUpgrader {
	counter, err := meter.Int64Counter("upgrade_in_progress", metric.WithDescription("Upgrade in progress"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeStarted, err := meter.Int64Counter("upgrade_started_total", metric.WithDescription("Total number of cluster upgrades started"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeCompleted, err := meter.Int64Counter("upgrade_completed_total", metric.WithDescription("Total number of cluster upgrades completed successfully"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeFailed, err := meter.Int64Counter("upgrade_failed_total", metric.WithDescription("Total number of cluster upgrades failed"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeStuck, err := meter.Int64Counter("upgrade_stuck_total", metric.WithDescription("Total number of cluster upgrades detected as stuck"))
	if err != nil {
		log.Fatal(err)
	}

	gkeApiCalls, err := meter.Int64Counter("gke_api_calls_total", metric.WithDescription("Total number of GKE API calls made"))
	if err != nil {
		log.Fatal(err)
	}

	gkeApiErrors, err := meter.Int64Counter("gke_api_errors_total", metric.WithDescription("Total number of GKE API errors encountered"))
	if err != nil {
		log.Fatal(err)
	}

	retryAttempts, err := meter.Int64Counter("retry_attempts_total", metric.WithDescription("Total number of retry attempts made"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeDuration, err := meter.Float64Histogram("upgrade_duration_seconds", metric.WithDescription("Duration of cluster upgrades in seconds"))
	if err != nil {
		log.Fatal(err)
	}

	return &ClusterUpgrader{
		log:               log,
		repo:              repo,
		client:            upgrader,
		upgradeInProgress: counter,
		upgradeStarted:    upgradeStarted,
		upgradeCompleted:  upgradeCompleted,
		upgradeFailed:     upgradeFailed,
		upgradeStuck:      upgradeStuck,
		gkeApiCalls:       gkeApiCalls,
		gkeApiErrors:      gkeApiErrors,
		retryAttempts:     retryAttempts,
		upgradeDuration:   upgradeDuration,
		slack:             slack,
		slackChannel:      slackChannel,
	}
}

func (c *ClusterUpgrader) Run(ctx context.Context) error {
	c.log.Info("starting cluster upgrader run")
	startTime := time.Now()

	var err error
	tenants, err := c.repo.TenantsGet(ctx)
	if err != nil {
		c.log.WithError(err).Error("failed to get tenants")
		return err
	}

	c.log.WithField("tenant_count", len(tenants)).Info("processing tenants for cluster upgrades")

	totalEnvironments := 0
	processedEnvironments := 0

	for _, tenant := range tenants {
		envs, err := c.repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			c.log.WithError(err).WithField("tenant", tenant.Name).Error("failed to get environments for tenant")
			return err
		}

		totalEnvironments += len(envs)

		for _, env := range envs {
			if err := c.upgradeEnvironment(ctx, tenant, env); err != nil {
				c.log.WithError(err).WithFields(logrus.Fields{
					"tenant":      tenant.Name,
					"environment": env.Name,
				}).Error("failed to process environment upgrade")
				return err
			}
			processedEnvironments++
			continue
		}
	}

	runDuration := time.Since(startTime)
	c.log.WithFields(logrus.Fields{
		"total_tenants":              len(tenants),
		"total_environments":         totalEnvironments,
		"processed_environments":     processedEnvironments,
		"run_duration_seconds":       runDuration.Seconds(),
		"avg_environment_processing": runDuration.Seconds() / float64(max(processedEnvironments, 1)),
	}).Info("cluster upgrader run completed")

	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (c *ClusterUpgrader) upgradeEnvironment(ctx context.Context, tenant *model.Tenant, env *model.Environment) error {
	log := c.log.WithFields(logrus.Fields{
		"tenant":      tenant.Name,
		"environment": env.Name,
		"tenant_id":   tenant.ID,
		"env_id":      env.ID,
	})

	log.Debug("checking environment for cluster upgrades")

	projectID, err := getProjectID(ctx, c, env.ID)
	if err != nil {
		log.WithError(err).Error("failed to get project ID for environment")
		return err
	}

	clusterUpgrade, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.WithError(err).Error("failed to get cluster upgrade status")
		return err
	}

	if clusterUpgrade == nil {
		log.Debug("no cluster upgrade found for environment")
		return nil
	}

	log = log.WithFields(logrus.Fields{
		"target_version": clusterUpgrade.Version,
		"current_status": clusterUpgrade.UpgradeStatus,
		"upgrade_id":     clusterUpgrade.ID,
		"last_modified":  clusterUpgrade.LastModified.Format("2006-01-02 15:04:05"),
	})

	log.Debug("processing cluster upgrade")

	// Check if upgrade is stuck (timeouts + GKE validation)
	if c.isUpgradeStuck(ctx, clusterUpgrade, projectID, env) {
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"status":         clusterUpgrade.UpgradeStatus,
			"last_modified":  clusterUpgrade.LastModified,
			"stuck_duration": time.Since(clusterUpgrade.LastModified).String(),
		}).Warn("cluster upgrade stuck, marking as failed")

		// Record stuck upgrade metric
		c.upgradeStuck.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, string(clusterUpgrade.UpgradeStatus))...))

		_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusFAILED, clusterUpgrade.Version)
		if err != nil {
			log.WithError(err).Error("failed to mark stuck upgrade as failed")
			return err
		}

		// Update local object to reflect the new status
		clusterUpgrade.UpgradeStatus = model.UpgradeStatusFailed

		// Record failed upgrade metric
		c.upgradeFailed.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "stuck_timeout")...))

		// Send Slack notification about stuck upgrade
		c.updateSlackProgress(tenant.Name, env.Name, clusterUpgrade)
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
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"tenant":         tenant.Name,
			"environment":    env.Name,
		}).Info("starting master upgrade")
		if clusterHas(runningOperations) {
			log.WithFields(logrus.Fields{"target_version": clusterUpgrade.Version}).Debug("has running operations, skipping...")
			return nil
		}

		// Record upgrade started metric
		c.upgradeStarted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "master")...))

		_, err = c.masterUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectID)
		if err != nil {
			// Record failed upgrade metric
			c.upgradeFailed.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "master_start_failed")...))
			return err
		}

		// Always update Slack progress with the current status
		c.updateSlackProgress(tenant.Name, env.Name, clusterUpgrade)

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

		// Update Slack with master completion
		c.updateSlackProgress(tenant.Name, env.Name, status)

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

				// Update Slack with nodepool start
				c.updateSlackProgress(tenant.Name, env.Name, un)
				return nil
			}

		}

		// node upgrade done, update status
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"tenant":         tenant.Name,
			"environment":    env.Name,
		}).Info("cluster upgrade completed successfully")

		upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatusDONE, clusterUpgrade.Version)
		if err != nil {
			return err
		}

		// Record successful completion metrics
		c.upgradeCompleted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "complete")...))

		// Record upgrade duration
		upgradeDuration := time.Since(clusterUpgrade.LastModified).Seconds()
		c.upgradeDuration.Record(ctx, upgradeDuration, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "total")...))

		// Update Slack with completion
		c.updateSlackProgress(tenant.Name, env.Name, upgradeStatus)

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

// postNewSlackMessage posts a new Slack message for the upgrade (used when metadata is missing)
func (c *ClusterUpgrader) postNewSlackMessage(tenantName, envName string, clusterUpgrade *model.ClusterUpgradeStatus) {
	mentions, err := getUpgradeMentions(context.Background(), c, clusterUpgrade.EnvironmentID)
	if err != nil {
		c.logNonCriticalError(err, "get_upgrade_mentions_fallback", logrus.Fields{
			"tenant":      tenantName,
			"environment": envName,
		})
		mentions = "" // Use empty mentions as fallback
	}

	startTime := clusterUpgrade.StartTime.Format("2006-01-02 15:04")
	msg := c.slack.GetClusterUpgradeProgressMessageOptions(
		tenantName,
		envName,
		clusterUpgrade.Version,
		clusterUpgrade.UpgradeStatus, // Pass UpgradeStatus directly as model type
		startTime,
		mentions)

	channelID, timestamp, err := c.slack.PostMessage(c.slackChannel, msg)
	if err != nil {
		c.logNonCriticalError(err, "slack_post_message_fallback", logrus.Fields{
			"tenant":      tenantName,
			"environment": envName,
			"version":     clusterUpgrade.Version,
			"status":      string(clusterUpgrade.UpgradeStatus),
		})
		return
	}

	// Save Slack metadata for future updates
	_, err = c.repo.SetClusterUpgradesSlackMessage(context.Background(), clusterUpgrade.ID, timestamp, channelID)
	if err != nil {
		c.logNonCriticalError(err, "set_slack_message_metadata_fallback", logrus.Fields{
			"tenant":             tenantName,
			"environment":        envName,
			"cluster_upgrade_id": clusterUpgrade.ID,
		})
	}
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

	// Record API call metric
	c.gkeApiCalls.Add(ctx, 1, metric.WithAttributes(attribute.String("operation", operation)))

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		err := fn()
		if err == nil {
			if attempt > 0 {
				c.log.WithFields(logrus.Fields{
					"operation":      operation,
					"attempt":        attempt + 1,
					"total_attempts": attempt + 1,
					"success":        true,
				}).Info("operation succeeded after retry")
			}
			return nil
		}

		lastErr = err

		// Record API error metric
		c.gkeApiErrors.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.Bool("retriable", isRetriableError(err)),
		))

		// Don't retry non-retriable errors
		if !isRetriableError(err) {
			c.log.WithFields(logrus.Fields{
				"operation": operation,
				"attempt":   attempt + 1,
				"retriable": false,
				"error":     err.Error(),
			}).Warn("non-retriable error, not retrying")
			return err
		}

		// Don't retry on last attempt
		if attempt == maxRetries {
			break
		}

		// Record retry attempt metric
		c.retryAttempts.Add(ctx, 1, metric.WithAttributes(
			attribute.String("operation", operation),
			attribute.Int("attempt", attempt+1),
		))

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
			"error":        err.Error(),
		}).Warn("retriable error, retrying after delay")

		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			// Continue to next attempt
		}
	}

	c.log.WithFields(logrus.Fields{
		"operation":      operation,
		"max_attempts":   maxRetries + 1,
		"total_attempts": maxRetries + 1,
		"final_error":    lastErr.Error(),
	}).Error("operation failed after all retry attempts")

	return lastErr
}

func (c *ClusterUpgrader) isUpgradeStuck(ctx context.Context, clusterUpgrade *model.ClusterUpgradeStatus, projectID string, env *model.Environment) bool {
	if clusterUpgrade.UpgradeStatus == model.UpgradeStatusDone || clusterUpgrade.UpgradeStatus == model.UpgradeStatusFailed {
		return false
	}

	return c.validateUpgradeAgainstGKE(ctx, clusterUpgrade, projectID, env)
}

func (c *ClusterUpgrader) validateUpgradeAgainstGKE(ctx context.Context, clusterUpgrade *model.ClusterUpgradeStatus, projectID string, env *model.Environment) bool {
	log := c.log.WithFields(logrus.Fields{
		"tenant":         env.TenantID,
		"environment":    env.Name,
		"upgrade_status": clusterUpgrade.UpgradeStatus,
		"upgrade_id":     clusterUpgrade.ID,
	})

	runningOperations, err := c.client.GetRunningOperations(ctx, projectID, env)
	if err != nil {
		log.WithError(err).Warn("failed to get running operations from GKE, assuming upgrade is not stuck")
		return false
	}

	// check if there are any upgrade-related operations running
	hasUpgradeOperations := false
	for _, op := range runningOperations {
		if op.OperationType == containerpb.Operation_UPGRADE_MASTER || op.OperationType == containerpb.Operation_UPGRADE_NODES {
			hasUpgradeOperations = true
			log.WithFields(logrus.Fields{
				"operation_name":   op.Name,
				"operation_type":   op.OperationType,
				"operation_status": op.Status,
			}).Debug("found running upgrade operation in GKE")
			break
		}
	}

	switch clusterUpgrade.UpgradeStatus {
	case model.UpgradeStatusCreated:
		// only consider stuck if it's been at least 30 minutes to avoid false positives
		if time.Since(clusterUpgrade.LastModified) > 30*time.Minute {
			log.WithField("duration_since_created", time.Since(clusterUpgrade.LastModified)).
				Debug("upgrade in CREATED status for >30min with no expected GKE operations - marking as stuck")
			return true
		}
		return false

	case model.UpgradeStatusMasterUpgrade:
		if !hasUpgradeOperations {
			log.Warn("upgrade in MASTER_UPGRADE status but no running master upgrade operations found in GKE - marking as stuck")
			return true
		}
		log.Debug("upgrade in MASTER_UPGRADE status with running GKE operations - not stuck")
		return false

	case model.UpgradeStatusNodeUpgrade:
		if !hasUpgradeOperations {
			log.Warn("upgrade in NODE_UPGRADE status but no running node upgrade operations found in GKE - marking as stuck")
			return true
		}
		log.Debug("upgrade in NODE_UPGRADE status with running GKE operations - not stuck")
		return false

	default:
		log.Debug("unknown upgrade status - not marking as stuck")
		return false
	}
}

// updateSlackProgress updates the existing Slack message with current upgrade progress
func (c *ClusterUpgrader) updateSlackProgress(tenantName, envName string, clusterUpgrade *model.ClusterUpgradeStatus) {
	if clusterUpgrade.SlackChannelID == "" || clusterUpgrade.SlackMessageTimestamp == "" {
		// No existing message - post a new one
		c.postNewSlackMessage(tenantName, envName, clusterUpgrade)
		return
	}

	// Retrieve mentions to maintain them through message updates
	mentions, err := getUpgradeMentions(context.Background(), c, clusterUpgrade.EnvironmentID)
	if err != nil {
		c.logNonCriticalError(err, "get_upgrade_mentions_update", logrus.Fields{
			"tenant":      tenantName,
			"environment": envName,
		})
		mentions = "" // Use empty mentions as fallback
	}

	startTime := clusterUpgrade.LastModified.Format("2006-01-02 15:04")
	progressMsg := c.slack.GetClusterUpgradeProgressMessageOptions(
		tenantName,
		envName,
		clusterUpgrade.Version,
		clusterUpgrade.UpgradeStatus, // Pass UpgradeStatus directly as model type
		startTime,
		mentions)

	_, _, _, err = c.slack.UpdateMessage(clusterUpgrade.SlackChannelID, clusterUpgrade.SlackMessageTimestamp, progressMsg)
	if err != nil {
		c.logNonCriticalError(err, "slack_update_progress", logrus.Fields{
			"tenant":      tenantName,
			"environment": envName,
			"status":      string(clusterUpgrade.UpgradeStatus),
		})
	}
}
