package upgrader

import (
	"context"
	"encoding/json"
	"errors"
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
	retryer      *Retryer

	// Metrics
	upgradeInProgress metric.Int64Counter
	upgradeStarted    metric.Int64Counter
	upgradeCompleted  metric.Int64Counter
	upgradeFailed     metric.Int64Counter
	upgradeStuck      metric.Int64Counter
	upgradeDuration   metric.Float64Histogram
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader Upgrader, meter metric.Meter, slack slack.SlackClient, slackChannel string) *ClusterUpgrader {
	counter, err := meter.Int64Counter("cluster_upgrade_in_progress", metric.WithDescription("Cluster upgrades currently in progress"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeStarted, err := meter.Int64Counter("cluster_upgrade_started", metric.WithDescription("Number of cluster upgrades started"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeCompleted, err := meter.Int64Counter("cluster_upgrade_completed", metric.WithDescription("Number of cluster upgrades completed successfully"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeFailed, err := meter.Int64Counter("cluster_upgrade_failed", metric.WithDescription("Number of cluster upgrades failed"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeStuck, err := meter.Int64Counter("cluster_upgrade_stuck", metric.WithDescription("Number of cluster upgrades detected as stuck"))
	if err != nil {
		log.Fatal(err)
	}

	gkeAPICalls, err := meter.Int64Counter("cluster_upgrader_gke_api_calls", metric.WithDescription("Number of GKE API calls made by cluster upgrader"))
	if err != nil {
		log.Fatal(err)
	}

	gkeAPIErrors, err := meter.Int64Counter("cluster_upgrader_gke_api_errors", metric.WithDescription("Number of GKE API errors encountered by cluster upgrader"))
	if err != nil {
		log.Fatal(err)
	}

	retryAttempts, err := meter.Int64Counter("cluster_upgrader_retry_attempts", metric.WithDescription("Number of retry attempts made by cluster upgrader"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeDuration, err := meter.Float64Histogram("cluster_upgrade_duration_seconds", metric.WithDescription("Duration of cluster upgrades in seconds"))
	if err != nil {
		log.Fatal(err)
	}

	retryer := NewRetryer(log, gkeAPICalls, gkeAPIErrors, retryAttempts, DefaultRetryConfig())

	return &ClusterUpgrader{
		log:               log,
		repo:              repo,
		client:            upgrader,
		upgradeInProgress: counter,
		upgradeStarted:    upgradeStarted,
		upgradeCompleted:  upgradeCompleted,
		upgradeFailed:     upgradeFailed,
		upgradeStuck:      upgradeStuck,
		upgradeDuration:   upgradeDuration,
		slack:             slack,
		slackChannel:      slackChannel,
		retryer:           retryer,
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
			// First clean up any dangling operations from completed upgrades
			if err := c.cleanupCompletedUpgradeOperations(ctx, tenant, env); err != nil {
				c.log.WithError(err).WithFields(logrus.Fields{
					"tenant":      tenant.Name,
					"environment": env.Name,
				}).Warn("failed to cleanup completed upgrade operations")
				// Don't fail the entire process for cleanup issues
			}

			// Then process active upgrades
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

	log.Debug("checking environment for cluster upgrades...")

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

	// Skip processing for completed upgrades - they're already done
	if clusterUpgrade.UpgradeStatus == model.UpgradeStatusDone || clusterUpgrade.UpgradeStatus == model.UpgradeStatusFailed {
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"status":         clusterUpgrade.UpgradeStatus,
		}).Debug("upgrade already completed, skipping")
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

		updatedStatus, err := c.masterUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectID)
		if err != nil {
			// Record failed upgrade metric
			c.upgradeFailed.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "master_start_failed")...))
			return err
		}

		// Always update Slack progress with the updated status
		c.updateSlackProgress(tenant.Name, env.Name, updatedStatus)

	case model.UpgradeStatusMasterUpgrade:
		// check status on ongoing master upgrade
		status, err := c.masterUpgradeStatus(ctx, env, clusterUpgrade, projectID, tenant.Name)
		if err != nil {
			return err
		}
		if status == nil {
			// upgrade not done - update Slack to refresh timestamp and show ongoing progress
			c.updateSlackProgress(tenant.Name, env.Name, clusterUpgrade)
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

	case model.UpgradeStatusDone:
		// Upgrade is already completed - this should have been handled earlier
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"status":         clusterUpgrade.UpgradeStatus,
		}).Debug("upgrade already completed")
		return nil

	case model.UpgradeStatusFailed:
		// Upgrade has failed - no further action needed
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"status":         clusterUpgrade.UpgradeStatus,
		}).Debug("upgrade failed, no action needed")
		return nil
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
	log := c.log.WithFields(logrus.Fields{
		"tenant_id":   env.TenantID,
		"environment": env.Name,
		"upgrade_id":  clusterUpgrade.ID,
	})

	// Get current running operations from GKE
	var runningOperations []*containerpb.Operation
	err := c.retryer.WithBackoff(ctx, "get_running_operations", func() error {
		var retryErr error
		runningOperations, retryErr = c.client.GetRunningOperations(ctx, projectID, env)
		return retryErr
	})
	if err != nil {
		return nil, err
	}

	// Get all existing operations for this upgrade from database
	existingOps, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
	if err != nil {
		log.WithError(err).Error("failed to get existing cluster operations")
		return nil, err
	}

	log.WithFields(logrus.Fields{
		"gke_running_operations": len(runningOperations),
		"db_existing_operations": len(existingOps),
	}).Debug("syncing cluster operations with GKE state")

	// Track which operations are still running
	runningOpNames := make(map[string]bool)

	// Update/create operations that are currently running in GKE
	for _, op := range runningOperations {
		if op.OperationType != containerpb.Operation_UPGRADE_NODES && op.OperationType != containerpb.Operation_UPGRADE_MASTER {
			continue
		}

		runningOpNames[op.Name] = true

		_, err := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
		if err != nil {
			log.WithError(err).WithField("operation", op.Name).Error("failed to update running operation")
			return nil, err
		}

		log.WithFields(logrus.Fields{
			"operation": op.Name,
			"status":    op.Status.String(),
			"type":      op.OperationType.String(),
		}).Debug("updated running operation")
	}

	// Check for operations that are no longer running in GKE and update their status
	for _, existingOp := range existingOps {
		// Skip operations that are already in a final state
		if existingOp.Status == "DONE" || existingOp.Status == "ABORTING" || existingOp.Status == "ABORTED" {
			log.WithFields(logrus.Fields{
				"operation": existingOp.Name,
				"status":    existingOp.Status,
			}).Debug("skipping operation already in final state")
			continue
		}

		if existingOp.Status != "RUNNING" {
			continue // Skip operations that are not in RUNNING state
		}

		if !runningOpNames[existingOp.Name] {
			// This operation is marked as RUNNING in our DB but not found in GKE
			// Need to fetch its current status from GKE
			log.WithField("operation", existingOp.Name).Debug("checking status of operation no longer running")

			err := c.retryer.WithBackoff(ctx, "get_operation_status", func() error {
				var gkeOp *containerpb.Operation
				var retryErr error
				gkeOp, retryErr = c.client.GetOperation(ctx, projectID, existingOp.Name)
				if retryErr != nil {
					return retryErr
				}

				// Update the operation with its final status
				_, updateErr := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, gkeOp)
				if updateErr != nil {
					log.WithError(updateErr).WithField("operation", existingOp.Name).Error("failed to update completed operation")
					return updateErr
				}

				log.WithFields(logrus.Fields{
					"operation":  existingOp.Name,
					"old_status": existingOp.Status,
					"new_status": gkeOp.Status.String(),
					"type":       gkeOp.OperationType.String(),
				}).Info("updated completed operation status")

				return nil
			})
			if err != nil {
				log.WithError(err).WithField("operation", existingOp.Name).Warn("failed to get operation status from GKE, operation may no longer exist")
				// Continue processing other operations rather than failing entirely
			}
		}
	}

	// Update cluster upgrade status to ensure consistency
	_, err = c.repo.UpdateClusterUpgradeStatus(ctx, env.TenantID, env.ID, gensql.ClusterUpgradesStatus(clusterUpgrade.UpgradeStatus), clusterUpgrade.Version)
	if err != nil {
		return nil, err
	}

	return runningOperations, nil
}

// cleanupDanglingOperations handles cleanup of stale operations for completed upgrades
func (c *ClusterUpgrader) cleanupDanglingOperations(ctx context.Context, projectID string, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus) error {
	log := c.log.WithFields(logrus.Fields{
		"tenant_id":      env.TenantID,
		"environment":    env.Name,
		"upgrade_id":     clusterUpgrade.ID,
		"upgrade_status": clusterUpgrade.UpgradeStatus,
	})

	// Get all existing operations for this completed upgrade from database
	existingOps, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
	if err != nil {
		log.WithError(err).Error("failed to get existing cluster operations for cleanup")
		return err
	}

	if len(existingOps) == 0 {
		log.Debug("no operations found for completed upgrade, nothing to cleanup")
		return nil
	}

	// Count operations that need cleanup
	danglingCount := 0
	for _, op := range existingOps {
		if op.Status == "RUNNING" {
			danglingCount++
		}
	}

	if danglingCount == 0 {
		log.WithField("total_operations", len(existingOps)).Debug("all operations for completed upgrade are already in final state")
		return nil
	}

	log.WithFields(logrus.Fields{
		"total_operations": len(existingOps),
		"dangling_running": danglingCount,
	}).Info("cleaning up dangling operations for completed upgrade")

	// Process each operation that might be dangling
	for _, existingOp := range existingOps {
		// Skip operations that are already in a final state
		if existingOp.Status == "DONE" || existingOp.Status == "ABORTING" || existingOp.Status == "ABORTED" {
			continue
		}

		if existingOp.Status == "RUNNING" {
			// Try to get the current status from GKE
			log.WithField("operation", existingOp.Name).Debug("checking final status of dangling operation")

			err := c.retryer.WithBackoff(ctx, "cleanup_operation_status", func() error {
				var gkeOp *containerpb.Operation
				var retryErr error
				gkeOp, retryErr = c.client.GetOperation(ctx, projectID, existingOp.Name)
				if retryErr != nil {
					return retryErr
				}

				// Update the operation with its final status
				_, updateErr := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, gkeOp)
				if updateErr != nil {
					log.WithError(updateErr).WithField("operation", existingOp.Name).Error("failed to update dangling operation")
					return updateErr
				}

				log.WithFields(logrus.Fields{
					"operation":    existingOp.Name,
					"old_status":   existingOp.Status,
					"final_status": gkeOp.Status.String(),
					"type":         gkeOp.OperationType.String(),
				}).Info("cleaned up dangling operation")

				return nil
			})
			if err != nil {
				log.WithError(err).WithField("operation", existingOp.Name).Warn("failed to get operation status from GKE during cleanup, operation may no longer exist")
				// Continue processing other operations rather than failing entirely
			}
		}
	}

	return nil
}

// cleanupCompletedUpgradeOperations cleans up dangling operations for completed upgrades
// This uses the history query to find ALL upgrades (including DONE/FAILED) and clean up their operations
func (c *ClusterUpgrader) cleanupCompletedUpgradeOperations(ctx context.Context, tenant *model.Tenant, env *model.Environment) error {
	log := c.log.WithFields(logrus.Fields{
		"tenant":      tenant.Name,
		"environment": env.Name,
	})

	// Get project ID for GKE API calls
	projectID, err := getProjectID(ctx, c, env.ID)
	if err != nil {
		log.WithError(err).Debug("failed to get project ID for cleanup, skipping")
		return nil // Don't fail cleanup for missing project ID
	}

	// Get ALL upgrades for this environment (including DONE/FAILED)
	allUpgrades, err := c.repo.ClusterUpgradeHistoryGet(ctx, env.TenantID, env.ID)
	if err != nil {
		log.WithError(err).Debug("failed to get upgrade history for cleanup")
		return err
	}

	// Only process completed upgrades that might have stale operations
	completedUpgrades := make([]*model.ClusterUpgradeStatus, 0)
	for _, upgrade := range allUpgrades {
		if upgrade.UpgradeStatus == model.UpgradeStatusDone || upgrade.UpgradeStatus == model.UpgradeStatusFailed {
			completedUpgrades = append(completedUpgrades, upgrade)
		}
	}

	if len(completedUpgrades) == 0 {
		log.Debug("no completed upgrades found for cleanup")
		return nil
	}

	log.WithField("completed_upgrades", len(completedUpgrades)).Debug("checking completed upgrades for dangling operations")

	// Clean up operations for each completed upgrade
	for _, upgrade := range completedUpgrades {
		err := c.cleanupDanglingOperations(ctx, projectID, env, upgrade)
		if err != nil {
			log.WithError(err).WithFields(logrus.Fields{
				"upgrade_id":     upgrade.ID,
				"upgrade_status": upgrade.UpgradeStatus,
				"version":        upgrade.Version,
			}).Warn("failed to cleanup operations for completed upgrade")
			// Continue with other upgrades
		}
	}

	return nil
}

func (c *ClusterUpgrader) upgradeNodes(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectID, tenantName string) (*model.ClusterUpgradeStatus, error) {
	var nodePools []*containerpb.NodePool
	err := c.retryer.WithBackoff(ctx, "get_node_pools", func() error {
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
		retryErr := c.retryer.WithBackoff(ctx, "upgrade_nodepool", func() error {
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
	err := c.retryer.WithBackoff(ctx, "upgrade_master", func() error {
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
	err := c.retryer.WithBackoff(ctx, "get_node_pools_status", func() error {
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
	err := c.retryer.WithBackoff(ctx, "get_operation_status", func() error {
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

// logNonCriticalError logs errors that don't stop the operation but should be noted
func (c *ClusterUpgrader) logNonCriticalError(err error, operation string, fields logrus.Fields) {
	if fields == nil {
		fields = logrus.Fields{}
	}
	fields["operation"] = operation
	fields["retriable"] = isRetriableError(err)

	c.log.WithFields(fields).WithError(err).Warn("non-critical operation failed")
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
				Warn("upgrade stuck in CREATED status")
			return true
		}
		return false

	case model.UpgradeStatusMasterUpgrade:
		if !hasUpgradeOperations {
			// Before marking as stuck, check if master upgrade actually completed
			if c.isMasterUpgradeComplete(ctx, clusterUpgrade, projectID, env, log) {
				return false
			}
			log.Warn("master upgrade stuck - no running operations in GKE")
			return true
		}
		return false

	case model.UpgradeStatusNodeUpgrade:
		if !hasUpgradeOperations {
			// Before marking as stuck, check if node upgrade actually completed
			if c.isNodeUpgradeComplete(ctx, clusterUpgrade, projectID, env, log) {
				return false
			}
			// Check if nodes need upgrading - if so, this is normal (not stuck)
			needsNodeUpgrade, err := c.checkIfNodesNeedUpgrade(ctx, clusterUpgrade, projectID, env)
			if err != nil {
				log.WithError(err).Debug("failed to check node upgrade status")
				return false
			}
			if needsNodeUpgrade {
				return false // Normal state - will start node upgrades
			}
			return false // Let main logic handle edge cases
		}
		return false

	default:
		log.Debug("unknown upgrade status - not marking as stuck")
		return false
	}
}

// isMasterUpgradeComplete checks if the master/apiserver upgrade has actually completed
// by comparing the current master version with the target version
func (c *ClusterUpgrader) isMasterUpgradeComplete(ctx context.Context, clusterUpgrade *model.ClusterUpgradeStatus, projectID string, env *model.Environment, log logrus.FieldLogger) bool {
	currentMasterVersion, err := c.client.GetCurrentMasterVersion(ctx, projectID, env)
	if err != nil {
		log.WithError(err).Warn("failed to get current master version, assuming upgrade not complete")
		return false
	}

	targetVersionObj, err := version.NewVersion(clusterUpgrade.Version)
	if err != nil {
		log.WithError(err).Warn("failed to parse target version, assuming upgrade not complete")
		return false
	}

	currentVersionObj, err := version.NewVersion(currentMasterVersion)
	if err != nil {
		log.WithError(err).Warn("failed to parse current master version, assuming upgrade not complete")
		return false
	}

	isComplete := currentVersionObj.GreaterThanOrEqual(targetVersionObj)
	if !isComplete {
		log.WithFields(logrus.Fields{
			"current_version": currentMasterVersion,
			"target_version":  clusterUpgrade.Version,
		}).Debug("master upgrade not yet complete")
	}

	return isComplete
}

// checkIfNodesNeedUpgrade checks if there are any node pools that need upgrading to the target version
func (c *ClusterUpgrader) checkIfNodesNeedUpgrade(ctx context.Context, clusterUpgrade *model.ClusterUpgradeStatus, projectID string, env *model.Environment) (bool, error) {
	nodePools, err := c.client.GetNodePools(ctx, projectID, env)
	if err != nil {
		return false, err
	}

	targetVersionObj, err := version.NewVersion(clusterUpgrade.Version)
	if err != nil {
		return false, err
	}

	for _, np := range nodePools {
		npVersionObj, err := version.NewVersion(np.Version)
		if err != nil {
			return false, err
		}

		if npVersionObj.LessThan(targetVersionObj) {
			return true, nil
		}
	}

	return false, nil
}

// isNodeUpgradeComplete checks if all node pools have been upgraded to the target version
func (c *ClusterUpgrader) isNodeUpgradeComplete(ctx context.Context, clusterUpgrade *model.ClusterUpgradeStatus, projectID string, env *model.Environment, log logrus.FieldLogger) bool {
	nodePools, err := c.client.GetNodePools(ctx, projectID, env)
	if err != nil {
		log.WithError(err).Warn("failed to get node pools, assuming upgrade not complete")
		return false
	}

	targetVersionObj, err := version.NewVersion(clusterUpgrade.Version)
	if err != nil {
		log.WithError(err).Warn("failed to parse target version, assuming upgrade not complete")
		return false
	}

	allComplete := true
	for _, np := range nodePools {
		npVersionObj, err := version.NewVersion(np.Version)
		if err != nil {
			log.WithError(err).WithField("nodepool", np.Name).Warn("failed to parse nodepool version")
			return false
		}

		if !npVersionObj.GreaterThanOrEqual(targetVersionObj) {
			allComplete = false
		}
	}

	return allComplete
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
