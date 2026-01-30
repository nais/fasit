package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
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
	client       ClusterManager
	slack        slack.SlackClient
	slackChannel string
	retryer      *Retryer

	// Metrics
	upgradeInProgress  metric.Int64Counter
	upgradeWaiting     metric.Int64Counter
	upgradeStarted     metric.Int64Counter
	upgradeCompleted   metric.Int64Counter
	upgradeFailed      metric.Int64Counter
	upgradeDuration    metric.Float64Histogram
	metricsInitialized bool
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, clusterManager ClusterManager, meter metric.Meter, slack slack.SlackClient, slackChannel string) *ClusterUpgrader {
	counter, err := meter.Int64Counter("cluster_upgrade_in_progress", metric.WithDescription("Cluster upgrades currently in progress"))
	if err != nil {
		log.Fatal(err)
	}

	upgradeWaiting, err := meter.Int64Counter("cluster_upgrade_waiting", metric.WithDescription("Cluster upgrades currently waiting due to delay configuration"))
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
		client:            clusterManager,
		upgradeInProgress: counter,
		upgradeWaiting:    upgradeWaiting,
		upgradeStarted:    upgradeStarted,
		upgradeCompleted:  upgradeCompleted,
		upgradeFailed:     upgradeFailed,
		upgradeDuration:   upgradeDuration,
		slack:             slack,
		slackChannel:      slackChannel,
		retryer:           retryer,
	}
}

func (c *ClusterUpgrader) Run(ctx context.Context) error {
	c.log.Debug("starting cluster upgrader run")
	startTime := time.Now()

	// Initialize metrics on first run by counting current upgrade states
	if err := c.initializeMetrics(ctx); err != nil {
		c.log.WithError(err).Warn("failed to initialize metrics from database state")
		// Don't fail the run, just log the warning
	}

	var err error
	tenants, err := c.repo.TenantsGet(ctx)
	if err != nil {
		c.log.WithError(err).Error("failed to get tenants")
		return err
	}

	c.log.WithField("tenant_count", len(tenants)).Debug("processing tenants for cluster upgrades")

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
	}).Debug("cluster upgrader run completed")

	return nil
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

	// Fetch current version once for this reconciliation cycle to avoid duplicate GKE API calls
	var currentVersion string
	err = c.retryer.WithBackoff(ctx, "get_current_control_plane_version", func() error {
		var retryErr error
		currentVersion, retryErr = c.client.GetCurrentControlPlaneVersion(ctx, projectID, env)
		return retryErr
	})
	if err != nil {
		log.WithError(err).Error("failed to get current control plane version")
		return err
	}

	// Check ownership BEFORE fetching running operations from GKE for all states except WAITING
	// Get running operations without tracking them yet to determine ownership
	if clusterUpgrade.UpgradeStatus == model.UpgradeStatusWaiting {
		existingOpsBeforeUpdate, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
		if err != nil {
			log.WithError(err).Error("failed to get existing operations for ownership check")
			return err
		}

		// Get running operations from GKE without tracking them yet
		runningOperations, err := c.getRunningOperationsFromGKE(ctx, projectID, env)
		if err != nil {
			return err
		}

		// Check for non-owned operations in all states except DONE/FAILED
		if clusterHas(runningOperations) && !c.ownsRunningOperations(existingOpsBeforeUpdate, runningOperations) {
			err := c.completeIfNonOwnedOperationsReachedTarget(ctx, projectID, env, tenant, clusterUpgrade, currentVersion, len(runningOperations))
			if err != nil {
				return err
			}
			// Non-owned operations detected - back off regardless of state
			return nil
		}

		// For WAITING upgrades, check if GKE has started operations before checking delay
		// This handles the case where GKE's auto-upgrade starts before Fasit's delay expires
		if clusterHas(runningOperations) {
			// We own these operations - track them and check if already complete
			runningOperations, completed, err := c.trackOwnedOperationsAndCheckCompletion(ctx, projectID, env, tenant, clusterUpgrade, currentVersion, runningOperations, existingOpsBeforeUpdate)
			if err != nil {
				return err
			}
			if completed {
				return nil
			}
			// We have operations in DB, so these running operations are ours - override delay and track them
			log.WithFields(logrus.Fields{
				"upgrade_id":      clusterUpgrade.ID,
				"target_version":  clusterUpgrade.Version,
				"current_version": currentVersion,
				"running_ops":     len(runningOperations),
			}).Warn("GKE started upgrade toward target version before delay expired, overriding delay to track operations")

			// Determine which operations are running and transition appropriately
			hasControlPlaneOp := false
			hasNodeOp := false
			for _, op := range runningOperations {
				if isOperationActive(op) {
					switch op.OperationType {
					case containerpb.Operation_UPGRADE_MASTER:
						hasControlPlaneOp = true
					case containerpb.Operation_UPGRADE_NODES:
						hasNodeOp = true
					}
				}
			}

			var targetStatus gensql.ClusterUpgradesStatus
			var metricsTarget string
			if hasNodeOp {
				targetStatus = gensql.ClusterUpgradesStatusNODEUPGRADE
				metricsTarget = "node_pools"
				log.Info("Transitioning to NODE_UPGRADE to track GKE-initiated upgrade")
			} else if hasControlPlaneOp {
				targetStatus = gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE
				metricsTarget = "control_plane"
				log.Info("Transitioning to CONTROL_PLANE_UPGRADE to track GKE-initiated upgrade")
			} else {
				log.Warn("Unknown operation type, staying in WAITING")
				return nil
			}

			upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, targetStatus)
			if err != nil {
				log.WithError(err).Error("failed to transition from WAITING to track GKE-initiated upgrade")
				return err
			}

			// Update metrics: decrement waiting, increment started and in-progress
			c.upgradeWaiting.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "waiting")...))
			c.upgradeStarted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, metricsTarget)...))
			c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, metricsTarget)...))

			c.updateSlackProgress(ctx, tenant.Name, env.Name, upgradeStatus)
			return nil
		}
	}

	// Check if upgrade should be delayed based on configuration
	if c.shouldDelayUpgrade(tenant, env, clusterUpgrade, log) {
		return nil
	}

	log.Debug("processing cluster upgrade")

	// Skip processing for completed upgrades - they're already done
	if clusterUpgrade.UpgradeStatus == model.UpgradeStatusDone || clusterUpgrade.UpgradeStatus == model.UpgradeStatusFailed {
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"status":         clusterUpgrade.UpgradeStatus,
		}).Debug("upgrade already completed, skipping")
		return nil
	}

	// Check ownership BEFORE fetching running operations from GKE for all non-WAITING states
	// Get running operations without tracking them yet to determine ownership
	existingOpsBeforeUpdate, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
	if err != nil {
		log.WithError(err).Error("failed to get existing operations for ownership check")
		return err
	}

	// Get running operations from GKE without tracking them yet
	runningOperations, err := c.getRunningOperationsFromGKE(ctx, projectID, env)
	if err != nil {
		return err
	}

	// Check for non-owned operations in all states except DONE/FAILED
	if clusterHas(runningOperations) && !c.ownsRunningOperations(existingOpsBeforeUpdate, runningOperations) {
		err := c.completeIfNonOwnedOperationsReachedTarget(ctx, projectID, env, tenant, clusterUpgrade, currentVersion, len(runningOperations))
		if err != nil {
			return err
		}
		// Non-owned operations detected - back off regardless of state
		return nil
	}

	// At this point, we own the operations (or there are none running)
	// Each state case below will track owned operations in their specific flow

	log.WithFields(logrus.Fields{"target_version": clusterUpgrade.Version, "status": clusterUpgrade.UpgradeStatus}).Debug("cluster upgrade status")
	switch clusterUpgrade.UpgradeStatus {
	case model.UpgradeStatusWaiting:
		// Note: Check for GKE-initiated operations is done earlier (before delay check)
		// If we reach here, delay period has passed and no operations are running
		// So we can proceed with starting the control plane upgrade

		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"tenant":         tenant.Name,
			"environment":    env.Name,
		}).Info("delay period satisfied, starting control plane upgrade")

		// Record metrics: decrement waiting, increment started and in-progress
		c.upgradeWaiting.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "waiting")...))
		c.upgradeStarted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "control_plane")...))
		c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "control_plane")...))

		updatedStatus, err := c.controlPlaneUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectID)
		if err != nil {
			// Record failed upgrade metric
			c.upgradeFailed.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "control_plane_start_failed")...))
			return err
		}

		// Always update Slack progress with the updated status
		c.updateSlackProgress(ctx, tenant.Name, env.Name, updatedStatus)

	case model.UpgradeStatusCreated:
		// Check if GKE has already started upgrade operations
		// At this point, ownership was already checked above, so if there are running ops, we own them
		if clusterHas(runningOperations) {
			runningOperations, completed, err := c.trackOwnedOperationsAndCheckCompletion(ctx, projectID, env, tenant, clusterUpgrade, currentVersion, runningOperations, existingOpsBeforeUpdate)
			if err != nil {
				return err
			}
			if completed {
				return nil
			}
			// Determine which operations are running
			hasControlPlaneOp := false
			hasNodeOp := false
			for _, op := range runningOperations {
				if isOperationActive(op) {
					switch op.OperationType {
					case containerpb.Operation_UPGRADE_MASTER:
						hasControlPlaneOp = true
					case containerpb.Operation_UPGRADE_NODES:
						hasNodeOp = true
					}
				}
			}

			// Transition to appropriate state based on running operations
			var targetStatus gensql.ClusterUpgradesStatus
			var metricsTarget string
			if hasNodeOp {
				// Node upgrade is running, skip to NODE_UPGRADE state
				targetStatus = gensql.ClusterUpgradesStatusNODEUPGRADE
				metricsTarget = "node_pools"
				log.WithFields(logrus.Fields{
					"upgrade_id":      clusterUpgrade.ID,
					"target_version":  clusterUpgrade.Version,
					"current_version": currentVersion,
				}).Info("GKE has already started node upgrade operations toward target version, transitioning to NODE_UPGRADE")
			} else if hasControlPlaneOp {
				// Control plane upgrade is running, transition to CONTROL_PLANE_UPGRADE state
				targetStatus = gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE
				metricsTarget = "control_plane"
				log.WithFields(logrus.Fields{
					"upgrade_id":      clusterUpgrade.ID,
					"target_version":  clusterUpgrade.Version,
					"current_version": currentVersion,
				}).Info("GKE has already started control plane upgrade operations toward target version, transitioning to CONTROL_PLANE_UPGRADE")
			} else {
				// Unknown operation type running, stay in CREATED and log
				log.WithFields(logrus.Fields{
					"upgrade_id":     clusterUpgrade.ID,
					"target_version": clusterUpgrade.Version,
					"running_ops":    len(runningOperations),
				}).Warn("GKE has unknown operations running, will check again next iteration")
				return nil
			}

			// Update status in database
			upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, targetStatus)
			if err != nil {
				log.WithError(err).Error("failed to transition upgrade status for GKE-initiated operations")
				return err
			}

			// Update metrics: increment started and in-progress
			c.upgradeStarted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, metricsTarget)...))
			c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, metricsTarget)...))

			// Update Slack progress
			c.updateSlackProgress(ctx, tenant.Name, env.Name, upgradeStatus)
			return nil
		} else if clusterUpgrade.IsAutomatic == nil || *clusterUpgrade.IsAutomatic {
			tenantDelay := tenant.UpgradeDelayDays
			envDelay := env.UpgradeDelayDays
			delayDays := tenantDelay + envDelay

			if delayDays > 0 {
				_, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusWAITING)
				if err != nil {
					log.WithError(err).Error("failed to update upgrade status to WAITING")
					return err
				}

				// Record waiting metric
				c.upgradeWaiting.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "waiting")...))

				log.WithFields(logrus.Fields{
					"upgrade_id":   clusterUpgrade.ID,
					"delay_days":   delayDays,
					"is_automatic": true,
				}).Info("upgrade status transitioned from CREATED to WAITING due to delay configuration")

				// Skip Slack notification for WAITING status - we'll notify when upgrade actually starts
				return nil
			}
		} else {
			log.WithFields(logrus.Fields{
				"upgrade_id":     clusterUpgrade.ID,
				"target_version": clusterUpgrade.Version,
				"is_automatic":   false,
			}).Info("manual upgrade bypassing delay configuration - proceeding immediately")
		}

		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"tenant":         tenant.Name,
			"environment":    env.Name,
		}).Info("starting control plane upgrade")

		// Use the already-fetched current version
		if currentVersion == clusterUpgrade.Version {
			log.WithFields(logrus.Fields{
				"target_version": clusterUpgrade.Version,
			}).Info("control plane already at target version, skipping to node upgrade")

			upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusNODEUPGRADE)
			if err != nil {
				return err
			}

			// Increment metrics for node upgrade phase (upgrade is starting)
			c.upgradeStarted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "node_pools")...))
			c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "node_pools")...))

			c.updateSlackProgress(ctx, tenant.Name, env.Name, upgradeStatus)
			return nil
		}

		// Record upgrade started metric
		c.upgradeStarted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "control_plane")...))

		updatedStatus, err := c.controlPlaneUpgrade(ctx, env, clusterUpgrade, tenant.Name, projectID)
		if err != nil {
			// Record failed upgrade metric
			c.upgradeFailed.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "control_plane_start_failed")...))
			return err
		}

		// Always update Slack progress with the updated status
		c.updateSlackProgress(ctx, tenant.Name, env.Name, updatedStatus)

	case model.UpgradeStatusControlPlaneUpgrade:
		// Track owned operations if any are running
		if clusterHas(runningOperations) {
			var hasActiveOps bool
			for _, op := range runningOperations {
				if isOperationActive(op) {
					hasActiveOps = true
					break
				}
			}
			if hasActiveOps {
				_, err = c.trackRunningOperations(ctx, projectID, env, clusterUpgrade, runningOperations, existingOpsBeforeUpdate)
				if err != nil {
					return err
				}
			}
		}

		// check status on ongoing control plane upgrade
		status, err := c.controlPlaneUpgradeStatus(ctx, env, clusterUpgrade, projectID, tenant.Name, currentVersion)
		if err != nil {
			return err
		}
		if status == nil {
			// upgrade not done - update Slack to refresh timestamp and show ongoing progress
			c.updateSlackProgress(ctx, tenant.Name, env.Name, clusterUpgrade)
			return nil
		}
		log.WithFields(logrus.Fields{"target_version": status.Version}).Info("control plane upgrade done")

		// Update Slack with control plane completion
		c.updateSlackProgress(ctx, tenant.Name, env.Name, status)

	case model.UpgradeStatusNodeUpgrade:
		// Track owned operations if any are running
		if clusterHas(runningOperations) {
			var hasActiveOps bool
			for _, op := range runningOperations {
				if isOperationActive(op) {
					hasActiveOps = true
					break
				}
			}
			if hasActiveOps {
				runningOperations, err = c.trackRunningOperations(ctx, projectID, env, clusterUpgrade, runningOperations, existingOpsBeforeUpdate)
				if err != nil {
					return err
				}
			}
		}

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
				c.updateSlackProgress(ctx, tenant.Name, env.Name, un)
			}
			// Upgrade not complete yet, return to retry on next reconciliation
			return nil
		}

		// node upgrade done, update status
		log.WithFields(logrus.Fields{
			"target_version": clusterUpgrade.Version,
			"tenant":         tenant.Name,
			"environment":    env.Name,
		}).Info("cluster upgrade completed successfully")

		upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusDONE)
		if err != nil {
			return err
		}

		// Record successful completion metrics
		c.upgradeCompleted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "complete")...))

		// Record upgrade duration (actual upgrade time, excluding WAITING period)
		if upgradeStatus.UpgradeStartTime != nil {
			upgradeDuration := upgradeStatus.LastModified.Sub(*upgradeStatus.UpgradeStartTime).Seconds()
			c.upgradeDuration.Record(ctx, upgradeDuration, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "total")...))
		} else {
			// Fallback for old upgrades without UpgradeStartTime
			c.log.WithFields(logrus.Fields{
				"upgrade_id": upgradeStatus.ID,
				"version":    upgradeStatus.Version,
			}).Warn("upgrade completed without UpgradeStartTime, skipping duration metric")
		}

		// Update Slack with completion
		c.updateSlackProgress(ctx, tenant.Name, env.Name, upgradeStatus)

		err = c.slack.AddReaction(upgradeStatus.SlackChannelID, upgradeStatus.SlackMessageTimestamp, "white_check_mark")
		if err != nil {
			log.WithError(err).WithFields(logrus.Fields{
				"tenant":      tenant.Name,
				"environment": env.Name,
				"reaction":    "white_check_mark",
			}).Warn("failed to add Slack reaction for completed upgrade - notification failure, upgrade completed successfully")
		}

		c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "node_pools")...))
		log.WithFields(logrus.Fields{
			"target_version": upgradeStatus.Version,
			"upgrade_id":     upgradeStatus.ID,
		}).Info("all nodepool upgrades completed successfully")

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

// ownsRunningOperations checks if running operations belong to this upgrade.
// Returns true if we have existing operations in DB for this upgrade AND those operations
// match operations currently running in GKE (by operation name).
//
// We check operation names to handle the scenario where:
// 1. Fasit initiates an upgrade and tracks operation A
// 2. GKE auto-upgrade starts operation B for a different version while A is still running
// 3. Only operation A should be considered "owned", not B
//
// Note on race conditions: There's a potential race between checking ClusterOperationsGetByUpgradeID
// and getting running operations from GKE where another upgrader instance could insert operations.
// This is mitigated by:
// 1. Checking operation names match (not just existence of any operations)
// 2. Database unique constraints preventing duplicate operation tracking
func (c *ClusterUpgrader) ownsRunningOperations(existingOps []*model.EnvironmentOperation, runningOps []*containerpb.Operation) bool {
	if len(existingOps) == 0 {
		return false
	}

	// Build map of existing operation names in database
	existingOpNames := make(map[string]bool)
	for _, op := range existingOps {
		existingOpNames[op.Name] = true
	}

	// Check if at least one running operation exists in our database
	// If GKE has started completely new operations (not in our DB), they're not ours
	for _, runningOp := range runningOps {
		if !isOperationActive(runningOp) {
			continue
		}
		if existingOpNames[runningOp.Name] {
			return true
		}
	}

	return false
}

// completeIfNonOwnedOperationsReachedTarget checks if cluster is at target version when non-owned operations exist,
// and marks upgrade as DONE if so. Otherwise logs a warning.
func (c *ClusterUpgrader) completeIfNonOwnedOperationsReachedTarget(
	ctx context.Context,
	projectID string,
	env *model.Environment,
	tenant *model.Tenant,
	clusterUpgrade *model.ClusterUpgradeStatus,
	currentVersion string,
	runningOpsCount int,
) error {
	log := c.log.WithFields(logrus.Fields{
		"tenant":      tenant.Name,
		"environment": env.Name,
		"upgrade_id":  clusterUpgrade.ID,
	})

	// Check if cluster is already at or beyond target version
	atTarget, err := c.isAtOrBeyondTargetVersion(clusterUpgrade, currentVersion)
	if err != nil {
		return err
	}

	if atTarget {
		// Mark upgrade as complete with all side effects
		return c.markUpgradeComplete(ctx, env, tenant, clusterUpgrade, runningOpsCount)
	}

	// Cluster not at target yet - log and back off
	log.WithFields(logrus.Fields{
		"upgrade_id":      clusterUpgrade.ID,
		"target_version":  clusterUpgrade.Version,
		"current_version": currentVersion,
		"running_ops":     runningOpsCount,
	}).Warn("GKE operations detected but no operations existed in DB before this run - these are not ours, backing off")

	return nil
}

// trackOwnedOperationsAndCheckCompletion tracks owned operations and checks if upgrade is complete.
// Returns (operations, completed, error).
func (c *ClusterUpgrader) trackOwnedOperationsAndCheckCompletion(
	ctx context.Context,
	projectID string,
	env *model.Environment,
	tenant *model.Tenant,
	clusterUpgrade *model.ClusterUpgradeStatus,
	currentVersion string,
	runningOps []*containerpb.Operation,
	existingOps []*model.EnvironmentOperation,
) ([]*containerpb.Operation, bool, error) {
	// Check if cluster is already at or beyond target version FIRST
	// This avoids unnecessary database writes if upgrade is already complete
	atTarget, err := c.isAtOrBeyondTargetVersion(clusterUpgrade, currentVersion)
	if err != nil {
		return nil, false, err
	}

	if atTarget {
		// Mark upgrade as complete without tracking operations
		err = c.markUpgradeComplete(ctx, env, tenant, clusterUpgrade, len(runningOps))
		if err != nil {
			return nil, false, err
		}
		return runningOps, true, nil
	}

	// Track the already-fetched operations in the database only if not yet complete
	runningOps, err = c.trackRunningOperations(ctx, projectID, env, clusterUpgrade, runningOps, existingOps)
	if err != nil {
		return nil, false, err
	}

	return runningOps, false, nil
}

// isAtOrBeyondTargetVersion checks if cluster version is at or beyond the target version.
// Returns (atTarget, error). Pure function with no side effects.
func (c *ClusterUpgrader) isAtOrBeyondTargetVersion(clusterUpgrade *model.ClusterUpgradeStatus, currentVersionStr string) (bool, error) {
	targetVer, err := version.NewVersion(clusterUpgrade.Version)
	if err != nil {
		return false, fmt.Errorf("failed to parse target version %q: %w", clusterUpgrade.Version, err)
	}

	currentVer, err := version.NewVersion(currentVersionStr)
	if err != nil {
		return false, fmt.Errorf("failed to parse current version %q: %w", currentVersionStr, err)
	}

	// Check if current version is at or beyond target
	atTarget := !currentVer.LessThan(targetVer)
	return atTarget, nil
}

// markUpgradeComplete marks an upgrade as DONE and performs all related side effects:
// updates database status, updates metrics, and notifies Slack.
func (c *ClusterUpgrader) markUpgradeComplete(ctx context.Context, env *model.Environment, tenant *model.Tenant, clusterUpgrade *model.ClusterUpgradeStatus, runningOpsCount int) error {
	log := c.log.WithFields(logrus.Fields{
		"tenant":      tenant.Name,
		"environment": env.Name,
		"upgrade_id":  clusterUpgrade.ID,
	})

	log.WithFields(logrus.Fields{
		"target_version": clusterUpgrade.Version,
		"running_ops":    runningOpsCount,
		"from_status":    clusterUpgrade.UpgradeStatus,
	}).Info("cluster at or beyond target version, marking upgrade as DONE")

	upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusDONE)
	if err != nil {
		return err
	}

	// Update metrics based on current state
	switch clusterUpgrade.UpgradeStatus {
	case model.UpgradeStatusWaiting:
		// Transitioning from WAITING to DONE - decrement waiting counter
		c.upgradeWaiting.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "waiting")...))
		c.upgradeCompleted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "complete")...))
	case model.UpgradeStatusControlPlaneUpgrade:
		// Transitioning from CONTROL_PLANE_UPGRADE to DONE - decrement in-progress counter for control plane
		c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "control_plane")...))
		c.upgradeCompleted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "complete")...))
	case model.UpgradeStatusNodeUpgrade:
		// Transitioning from NODE_UPGRADE to DONE - decrement in-progress counter for node pools
		c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "node_pools")...))
		c.upgradeCompleted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "complete")...))
	case model.UpgradeStatusCreated:
		// Transitioning from CREATED to DONE - no metrics to decrement since upgrade never started
		// The cluster was already upgraded by GKE auto-upgrade or another mechanism before Fasit could act
		// We only increment upgradeCompleted to track that the upgrade goal was achieved
		c.upgradeCompleted.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "complete")...))
	}

	// Update Slack
	c.updateSlackProgress(ctx, tenant.Name, env.Name, upgradeStatus)
	err = c.slack.AddReaction(upgradeStatus.SlackChannelID, upgradeStatus.SlackMessageTimestamp, "white_check_mark")
	if err != nil {
		log.WithError(err).WithFields(logrus.Fields{
			"tenant":      tenant.Name,
			"environment": env.Name,
			"reaction":    "white_check_mark",
		}).Warn("failed to add Slack reaction for completed upgrade - notification failure, upgrade completed successfully")
	}

	return nil
}

// isOperationActive checks if an operation is in an active state (RUNNING or PENDING)
func isOperationActive(op *containerpb.Operation) bool {
	return op.Status == containerpb.Operation_RUNNING || op.Status == containerpb.Operation_PENDING
}

// extractNodePoolName extracts the nodepool name from a GKE target link
func extractNodePoolName(targetLink string) string {
	// Target link format: https://container.googleapis.com/v1/projects/{project}/locations/{location}/clusters/{cluster}/nodePools/{nodepool}

	// Strip query parameters and fragments if present
	if i := strings.IndexAny(targetLink, "?#"); i != -1 {
		targetLink = targetLink[:i]
	}

	parts := strings.Split(targetLink, "/")
	for i := 0; i < len(parts)-1; i++ {
		if parts[i] == "nodePools" {
			return parts[i+1]
		}
	}
	return ""
}

// containsNodePool checks if the target link references the specified nodepool name as a distinct path segment
func containsNodePool(targetLink, nodepoolName string) bool {
	// Target link format: https://container.googleapis.com/v1/projects/{project}/locations/{location}/clusters/{cluster}/nodePools/{nodepool}

	// Empty nodepool name is not valid
	if nodepoolName == "" {
		return false
	}

	return extractNodePoolName(targetLink) == nodepoolName
}

// latestNodepoolUpgradeOps builds a map of the most recent UPGRADE_NODES operation for each nodepool
func latestNodepoolUpgradeOps(ops []*model.EnvironmentOperation) map[string]*model.EnvironmentOperation {
	latestOps := make(map[string]*model.EnvironmentOperation)
	for _, op := range ops {
		if op.Type != "UPGRADE_NODES" || op.Target == "" {
			continue // Skip non-nodepool operations
		}

		// Extract nodepool name from target URL
		nodepoolName := extractNodePoolName(op.Target)
		if nodepoolName == "" {
			continue // Skip if we can't extract the nodepool name
		}

		existing, exists := latestOps[nodepoolName]
		if !exists || op.LastModified.After(existing.LastModified) {
			latestOps[nodepoolName] = op
		}
	}
	return latestOps
}

func clusterHas(runningOperations []*containerpb.Operation) bool {
	for _, op := range runningOperations {
		if isOperationActive(op) {
			return true
		}
	}
	return false
}

// getRunningOperationsFromGKE retrieves running operations from GKE without updating the database
func (c *ClusterUpgrader) getRunningOperationsFromGKE(ctx context.Context, projectID string, env *model.Environment) ([]*containerpb.Operation, error) {
	var runningOperations []*containerpb.Operation
	err := c.retryer.WithBackoff(ctx, "get_running_operations", func() error {
		var retryErr error
		runningOperations, retryErr = c.client.GetRunningOperations(ctx, projectID, env)
		return retryErr
	})
	if err != nil {
		return nil, err
	}
	return runningOperations, nil
}

// trackRunningOperations updates the database with already-fetched running operations.
// Returns the updated list of operations after tracking completion status.
// Accepts existingOps to avoid duplicate database queries.
func (c *ClusterUpgrader) trackRunningOperations(
	ctx context.Context,
	projectID string,
	env *model.Environment,
	clusterUpgrade *model.ClusterUpgradeStatus,
	runningOperations []*containerpb.Operation,
	existingOps []*model.EnvironmentOperation,
) ([]*containerpb.Operation, error) {
	log := c.log.WithFields(logrus.Fields{
		"tenant_id":   env.TenantID,
		"environment": env.Name,
		"upgrade_id":  clusterUpgrade.ID,
	})

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

		// Only process operations that are RUNNING or PENDING
		if existingOp.Status != "RUNNING" && existingOp.Status != "PENDING" {
			continue
		}

		if !runningOpNames[existingOp.Name] {
			// This operation is marked as RUNNING or PENDING in our DB but not found in GKE
			// Need to fetch its current status from GKE (might have completed)
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

	return runningOperations, nil
}

// cleanupCompletedUpgradeOperations cleans up dangling operations for completed upgrades
// Uses a SQL query to efficiently find all RUNNING operations for DONE/FAILED upgrades
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

	// Get all RUNNING operations for completed (DONE/FAILED) upgrades in this environment
	// This query uses a JOIN to efficiently find dangling operations without fetching all upgrades
	// Returns a map of upgrade_id -> operations for that upgrade
	danglingOpsByUpgrade, err := c.repo.ClusterOperationsGetDanglingForEnvironment(ctx, env.TenantID, env.ID)
	if err != nil {
		log.WithError(err).Debug("failed to get dangling operations for cleanup")
		return err
	}

	if len(danglingOpsByUpgrade) == 0 {
		log.Debug("no dangling operations found for cleanup")
		return nil
	}

	totalOps := 0
	for _, ops := range danglingOpsByUpgrade {
		totalOps += len(ops)
	}

	log.WithFields(logrus.Fields{
		"affected_upgrades":   len(danglingOpsByUpgrade),
		"dangling_operations": totalOps,
	}).Info("cleaning up dangling operations for completed upgrades")

	// Process operations grouped by upgrade
	for upgradeID, ops := range danglingOpsByUpgrade {
		for _, op := range ops {
			log.WithFields(logrus.Fields{
				"operation":  op.Name,
				"upgrade_id": upgradeID,
			}).Debug("checking final status of dangling operation")

			err := c.retryer.WithBackoff(ctx, "cleanup_operation_status", func() error {
				var gkeOp *containerpb.Operation
				var retryErr error
				gkeOp, retryErr = c.client.GetOperation(ctx, projectID, op.Name)
				if retryErr != nil {
					return retryErr
				}

				// Update the operation with its final status
				_, updateErr := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, upgradeID, gkeOp)
				if updateErr != nil {
					log.WithError(updateErr).WithField("operation", op.Name).Error("failed to update dangling operation")
					return updateErr
				}

				log.WithFields(logrus.Fields{
					"operation":    op.Name,
					"old_status":   op.Status,
					"final_status": gkeOp.Status.String(),
					"type":         gkeOp.OperationType.String(),
				}).Info("cleaned up dangling operation")

				return nil
			})
			if err != nil {
				// Check if operation no longer exists in GKE (NotFound error)
				if apiErr, ok := err.(*apierror.APIError); ok && apiErr.GRPCStatus().Code() == codes.NotFound {
					// Operation doesn't exist in GKE anymore - mark it as DONE in our database
					log.WithFields(logrus.Fields{
						"operation":  op.Name,
						"upgrade_id": upgradeID,
					}).Info("operation no longer exists in GKE, marking as DONE")

					// Create a synthetic DONE operation to update the database
					doneOp := &containerpb.Operation{
						Name:   op.Name,
						Status: containerpb.Operation_DONE,
					}
					_, updateErr := c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, upgradeID, doneOp)
					if updateErr != nil {
						log.WithError(updateErr).WithField("operation", op.Name).Error("failed to mark stale operation as DONE")
						// Continue processing other operations
					}
				} else {
					log.WithError(err).WithField("operation", op.Name).Warn("failed to get operation status from GKE during cleanup")
				}
				// Continue processing other operations rather than failing entirely
			}
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

	// Get existing operations to avoid creating duplicates
	existingOps, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
	if err != nil {
		c.log.WithError(err).Error("failed to get existing cluster operations")
		return nil, err
	}

	// Build a map of latest operation per nodepool for efficient lookup
	latestOpPerNodepool := latestNodepoolUpgradeOps(existingOps)

	if len(latestOpPerNodepool) > 0 {
		opsInfo := make(map[string]string)
		for k, v := range latestOpPerNodepool {
			opsInfo[k] = v.Status
		}
		c.log.WithFields(logrus.Fields{
			"upgrade_id":             clusterUpgrade.ID,
			"nodepool_count":         len(nodePools),
			"operations_by_nodepool": opsInfo,
		}).Info("processing node pools with existing operations")
	}

	for _, np := range nodePools {
		npVersionObj, err := version.NewVersion(np.Version)
		if err != nil {
			return nil, err
		}

		// Check the latest operation for this nodepool
		hasOperation := false
		needsRetry := false
		if latestOp, exists := latestOpPerNodepool[np.Name]; exists {
			// Check if this operation is active (PENDING or RUNNING)
			if latestOp.Status == "PENDING" || latestOp.Status == "RUNNING" {
				hasOperation = true
				c.log.WithFields(logrus.Fields{
					"nodepool":  np.Name,
					"operation": latestOp.Name,
					"status":    latestOp.Status,
				}).Debug("nodepool already has an operation, skipping new upgrade")
			} else if (latestOp.Status == "DONE" && latestOp.NodesFailed > 0) || latestOp.Status == "ABORTED" || latestOp.Status == "ABORTING" {
				// Check if this is a recently failed operation (apply backoff to avoid hammering GCP)
				timeSinceFailure := time.Since(latestOp.LastModified)
				backoffDuration := 30 * time.Minute
				if timeSinceFailure < backoffDuration {
					hasOperation = true
					c.log.WithFields(logrus.Fields{
						"nodepool":        np.Name,
						"operation":       latestOp.Name,
						"status":          latestOp.Status,
						"nodes_failed":    latestOp.NodesFailed,
						"time_since_fail": timeSinceFailure,
						"retry_available": backoffDuration - timeSinceFailure,
					}).Info("nodepool upgrade recently failed, applying backoff before retry (will continue with other nodepools)")
				} else {
					// Backoff period has passed, this nodepool needs a retry even if version matches
					needsRetry = true
					c.log.WithFields(logrus.Fields{
						"nodepool":        np.Name,
						"operation":       latestOp.Name,
						"nodes_failed":    latestOp.NodesFailed,
						"time_since_fail": timeSinceFailure,
					}).Info("nodepool upgrade previously failed, will retry now")
				}
			}
		} else if len(latestOpPerNodepool) > 0 {
			// We have operations but none matched this nodepool name - log for debugging
			c.log.WithFields(logrus.Fields{
				"nodepool":   np.Name,
				"upgrade_id": clusterUpgrade.ID,
				"available_pools": func() []string {
					keys := make([]string, 0, len(latestOpPerNodepool))
					for k := range latestOpPerNodepool {
						keys = append(keys, k)
					}
					return keys
				}(),
			}).Info("nodepool not found in operations map - name mismatch?")
		}

		if hasOperation {
			continue
		}

		// Skip if nodepool is already at target version and doesn't need retry
		if npVersionObj.GreaterThanOrEqual(clusterUpgraderVersionObj) && !needsRetry {
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

		_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, clusterUpgrade.ID, op)
		if err != nil {
			return nil, err
		}

		us, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatus(clusterUpgrade.UpgradeStatus))
		if err != nil {
			return nil, err
		}
		c.log.WithFields(logrus.Fields{
			"tenant":      tenantName,
			"environment": env.Name,
			"nodepool":    np.Name,
			"version":     clusterUpgrade.Version,
		}).Info("started upgrade of nodepool")
		return us, nil
	}
	return nil, nil
}

func (c *ClusterUpgrader) nodeUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectID string) (bool, error) {
	rop, err := c.repo.GetActiveClusterOperation(ctx, env.TenantID, env.ID)
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

func (c *ClusterUpgrader) controlPlaneUpgradeStatus(ctx context.Context, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, projectID, tenantName, currentVersion string) (*model.ClusterUpgradeStatus, error) {
	rop, err := c.repo.GetActiveClusterOperation(ctx, env.TenantID, env.ID)
	if err != nil {
		return nil, err
	}

	// Ignore UPGRADE_NODES operations - we only care about UPGRADE_MASTER for control plane status
	if rop != nil && rop.Type != "UPGRADE_MASTER" {
		rop = nil
	}

	if rop == nil {
		// No RUNNING operation found - check if there's a completed one or verify with GKE
		c.log.WithFields(logrus.Fields{
			"tenant":      tenantName,
			"environment": env.Name,
			"upgrade_id":  clusterUpgrade.ID,
		}).Debug("no running operation in database, checking for completed operations")

		// Check if there are any operations for this upgrade (including DONE ones)
		ops, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
		if err != nil {
			return nil, err
		}

		// If we have operations, check if any are DONE
		var doneOp *model.EnvironmentOperation
		for _, op := range ops {
			if op.Status == "DONE" {
				doneOp = op
				break
			}
		}

		if doneOp != nil {
			// We have a DONE operation, transition to NODE_UPGRADE
			c.log.WithFields(logrus.Fields{
				"tenant":         tenantName,
				"environment":    env.Name,
				"operation_name": doneOp.Name,
			}).Debug("found completed control plane upgrade operation, transitioning to node upgrade")

			c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(
				setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "control_plane")...),
			)

			upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusNODEUPGRADE)
			if err != nil {
				return nil, err
			}
			c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("control plane upgrade to %s done", upgradeStatus.Version)
			return upgradeStatus, nil
		}

		// No operations at all - verify with GKE directly using already-fetched version
		c.log.WithFields(logrus.Fields{
			"tenant":      tenantName,
			"environment": env.Name,
		}).Warn("no operations found in database for control plane upgrade, verifying with already-fetched version")

		// If the control plane is already at the target version, mark it as complete
		if currentVersion == clusterUpgrade.Version {
			c.log.WithFields(logrus.Fields{
				"tenant":         tenantName,
				"environment":    env.Name,
				"target_version": clusterUpgrade.Version,
			}).Info("control plane already at target version, marking upgrade as complete")

			// Adjust metrics: control plane complete, starting node upgrade
			c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "control_plane")...))
			c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "node_pools")...))

			upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusNODEUPGRADE)
			if err != nil {
				return nil, err
			}
			return upgradeStatus, nil
		}

		// Control plane not at target version and no running operation - stuck
		c.log.WithFields(logrus.Fields{
			"tenant":          tenantName,
			"environment":     env.Name,
			"current_version": currentVersion,
			"target_version":  clusterUpgrade.Version,
		}).Warn("control plane upgrade stuck - not at target version and no running operation")
		return nil, nil
	}

	op, err := c.getAndUpdateOperation(ctx, projectID, env.TenantID, env.ID, clusterUpgrade.ID, rop.Name)
	if err != nil {
		return nil, err
	}

	var upgradeStatus *model.ClusterUpgradeStatus
	if op.Status == containerpb.Operation_DONE {
		c.upgradeInProgress.Add(ctx, -1, metric.WithAttributes(
			setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "control_plane")...),
		)
		c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(
			setMetricsAttrs(env.Name, tenantName, clusterUpgrade.Version, "node_pools")...),
		)
		upgradeStatus, err = c.repo.UpdateClusterUpgradeStatus(ctx, clusterUpgrade.ID, gensql.ClusterUpgradesStatusNODEUPGRADE)
		if err != nil {
			return nil, err
		}
		c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("control plane upgrade to %s done", upgradeStatus.Version)
	}
	return upgradeStatus, nil
}

func (c *ClusterUpgrader) controlPlaneUpgrade(ctx context.Context, env *model.Environment, upgrade *model.ClusterUpgradeStatus, tenantName, projectID string) (*model.ClusterUpgradeStatus, error) {
	var op *containerpb.Operation

	// Retry the GKE API call with exponential backoff
	err := c.retryer.WithBackoff(ctx, "upgrade_control_plane", func() error {
		var retryErr error
		op, retryErr = c.client.UpgradeControlPlane(ctx, projectID, env, upgrade.Version)
		return retryErr
	})
	if err != nil {
		if e, ok := err.(*apierror.APIError); ok {
			if e.GRPCStatus().Code() == codes.InvalidArgument {
				c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("invalid argument: %s", e.GRPCStatus().Message())
				_, err = c.repo.UpdateClusterUpgradeStatus(ctx, upgrade.ID, gensql.ClusterUpgradesStatusFAILED)
				if err != nil {
					return nil, err
				}
			}
		}
		return nil, err
	}

	_, err = c.repo.CreateOrUpdateClusterOperation(ctx, env.TenantID, env.ID, upgrade.ID, op)
	if err != nil {
		return nil, err
	}

	upgradeStatus, err := c.repo.UpdateClusterUpgradeStatus(ctx, upgrade.ID, gensql.ClusterUpgradesStatusCONTROLPLANEUPGRADE)
	if err != nil {
		return upgradeStatus, err
	}
	c.log.WithFields(logrus.Fields{"tenant": tenantName, "environment": env.Name}).Infof("control plane upgrade to %s started", upgradeStatus.Version)
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

	// Check if all nodepool versions match the target
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

	// Even if GKE reports nodepools at target version, check if operations actually succeeded
	// GKE sometimes reports target version before individual nodes are upgraded
	if done {
		ops, err := c.repo.ClusterOperationsGetByUpgradeID(ctx, clusterUpgrade.ID)
		if err != nil {
			c.log.WithError(err).Warn("failed to get operations for completion check, will retry on next reconciliation")
			return false, nil
		}

		// Find the most recent operation for each nodepool
		latestOps := latestNodepoolUpgradeOps(ops)

		// Check if any nodepool's latest operation has failed nodes or is still in progress
		failedNodepools := []string{}
		incompleteNodepools := []string{}
		totalNodesFailedCount := 0
		totalFailedOperations := 0
		for nodepoolName, op := range latestOps {
			// Check for failed operations
			if (op.Status == "DONE" && op.NodesFailed > 0) || op.Status == "ABORTED" || op.Status == "ABORTING" {
				failedNodepools = append(failedNodepools, nodepoolName)
				if op.NodesFailed > 0 {
					totalNodesFailedCount += op.NodesFailed
				}
			} else if op.Status == "RUNNING" || op.Status == "PENDING" {
				// Operation still in progress - not complete yet
				incompleteNodepools = append(incompleteNodepools, nodepoolName)
			}
		}

		// Count total failed operations across all nodepools (including historical retries)
		if len(failedNodepools) > 0 {
			for _, op := range ops {
				if op.Type != "UPGRADE_NODES" {
					continue
				}
				if (op.Status == "DONE" && op.NodesFailed > 0) || op.Status == "ABORTED" || op.Status == "ABORTING" {
					totalFailedOperations++
				}
			}
		}

		if len(failedNodepools) > 0 {
			c.log.WithFields(logrus.Fields{
				"upgrade_id":               clusterUpgrade.ID,
				"failed_nodepools":         failedNodepools,
				"failed_nodepool_count":    len(failedNodepools),
				"failed_operation_count":   totalFailedOperations,
				"total_nodes_failed_count": totalNodesFailedCount,
				"total_nodepools":          len(nodepools),
			}).Info("upgrade not complete - latest operations have failed nodes, will retry after backoff")
			return false, nil
		}

		if len(incompleteNodepools) > 0 {
			c.log.WithFields(logrus.Fields{
				"upgrade_id":           clusterUpgrade.ID,
				"incomplete_nodepools": incompleteNodepools,
				"incomplete_count":     len(incompleteNodepools),
				"total_nodepools":      len(nodepools),
			}).Debug("upgrade not complete - operations still running")
			return false, nil
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
func (c *ClusterUpgrader) postNewSlackMessage(ctx context.Context, tenantName, envName string, clusterUpgrade *model.ClusterUpgradeStatus) {
	mentions, err := getUpgradeMentions(ctx, c, clusterUpgrade.EnvironmentID)
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
	_, err = c.repo.SetClusterUpgradesSlackMessage(ctx, clusterUpgrade.ID, timestamp, channelID)
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

// shouldDelayUpgrade checks if an upgrade should be delayed based on upgrade_delay_days configuration.
// Only processes WAITING status - delay check returns true if still waiting, false if ready to proceed.
// Delay is additive: tenant delay + environment delay (default 0 for each).
func (c *ClusterUpgrader) shouldDelayUpgrade(tenant *model.Tenant, env *model.Environment, clusterUpgrade *model.ClusterUpgradeStatus, log logrus.FieldLogger) bool {
	if clusterUpgrade.UpgradeStatus != model.UpgradeStatusWaiting {
		return false
	}

	tenantDelay := tenant.UpgradeDelayDays
	envDelay := env.UpgradeDelayDays
	delayDays := tenantDelay + envDelay

	// No delay configured - ready to proceed
	if delayDays == 0 {
		return false
	}

	requiredDelayHours := time.Duration(delayDays) * 24 * time.Hour
	timeSinceCreation := time.Since(clusterUpgrade.StartTime)

	// Determine delay source for logging
	delaySource := "default"
	if tenantDelay != 0 && envDelay != 0 {
		delaySource = "tenant+environment"
	} else if tenantDelay != 0 {
		delaySource = "tenant"
	} else if envDelay != 0 {
		delaySource = "environment"
	}

	if timeSinceCreation < requiredDelayHours {
		remainingDelay := requiredDelayHours - timeSinceCreation
		log.WithFields(logrus.Fields{
			"delay_days":        delayDays,
			"delay_source":      delaySource,
			"required_delay":    requiredDelayHours.String(),
			"time_since_create": timeSinceCreation.String(),
			"remaining_delay":   remainingDelay.String(),
			"will_start_at":     clusterUpgrade.StartTime.Add(requiredDelayHours).Format("2006-01-02 15:04:05"),
		}).Info("upgrade waiting for delay_days to pass")
		return true
	}

	// Delay period has passed - check if we're in business hours
	location, err := time.LoadLocation("Europe/Oslo")
	if err != nil {
		log.WithError(err).Warn("failed to load Oslo timezone, proceeding with upgrade anyway")
		return false
	}
	now := time.Now().In(location)

	if !IsBusinessHours() {
		log.WithFields(logrus.Fields{
			"delay_days":     delayDays,
			"delay_source":   delaySource,
			"waited_for":     timeSinceCreation.String(),
			"current_time":   now.Format("2006-01-02 15:04:05"),
			"current_day":    now.Weekday().String(),
			"business_hours": "Mon-Fri 9-16 Oslo time",
		}).Info("delay satisfied but waiting for business hours to start upgrade")
		return true
	}

	log.WithFields(logrus.Fields{
		"delay_days":     delayDays,
		"delay_source":   delaySource,
		"required_delay": requiredDelayHours.String(),
		"waited_for":     timeSinceCreation.String(),
		"current_time":   now.Format("2006-01-02 15:04:05"),
	}).Info("delay_days satisfied and within business hours, proceeding with upgrade")
	return false
}

// updateSlackProgress updates the existing Slack message with current upgrade progress
func (c *ClusterUpgrader) updateSlackProgress(ctx context.Context, tenantName, envName string, clusterUpgrade *model.ClusterUpgradeStatus) {
	if clusterUpgrade.SlackChannelID == "" || clusterUpgrade.SlackMessageTimestamp == "" {
		// No existing message - post a new one
		c.postNewSlackMessage(ctx, tenantName, envName, clusterUpgrade)
		return
	}

	// Retrieve mentions to maintain them through message updates
	mentions, err := getUpgradeMentions(ctx, c, clusterUpgrade.EnvironmentID)
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

// initializeMetrics sets metric values based on current database state
// This is called once on the first Run() to ensure metrics reflect reality after restart
func (c *ClusterUpgrader) initializeMetrics(ctx context.Context) error {
	if c.metricsInitialized {
		return nil
	}

	c.log.Debug("initializing metrics from database state")

	tenants, err := c.repo.TenantsGet(ctx)
	if err != nil {
		return err
	}

	waitingCount := 0
	inProgressCount := 0

	for _, tenant := range tenants {
		envs, err := c.repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			c.log.WithError(err).WithField("tenant", tenant.Name).Warn("failed to get environments for metric initialization")
			continue
		}

		for _, env := range envs {
			clusterUpgrade, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
			if err != nil {
				if !errors.Is(err, pgx.ErrNoRows) {
					c.log.WithError(err).WithFields(logrus.Fields{
						"tenant":      tenant.Name,
						"environment": env.Name,
					}).Warn("failed to get cluster upgrade for metric initialization")
				}
				continue
			}

			if clusterUpgrade == nil {
				continue
			}

			switch clusterUpgrade.UpgradeStatus {
			case model.UpgradeStatusWaiting:
				waitingCount++
				c.upgradeWaiting.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, "waiting")...))
			case model.UpgradeStatusControlPlaneUpgrade, model.UpgradeStatusNodeUpgrade:
				inProgressCount++
				target := "control_plane"
				if clusterUpgrade.UpgradeStatus == model.UpgradeStatusNodeUpgrade {
					target = "node_pools"
				}
				c.upgradeInProgress.Add(ctx, 1, metric.WithAttributes(setMetricsAttrs(env.Name, tenant.Name, clusterUpgrade.Version, target)...))
			}
		}
	}

	c.log.WithFields(logrus.Fields{
		"waiting_upgrades":     waitingCount,
		"in_progress_upgrades": inProgressCount,
	}).Info("metrics initialized from database state")

	c.metricsInitialized = true
	return nil
}
