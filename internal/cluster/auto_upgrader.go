package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type AutoUpgrader struct {
	repo                      database.Repo
	log                       logrus.FieldLogger
	client                    ClusterManager
	autoUpgradesScheduled     metric.Int64Counter
	autoUpgradeProcessingTime metric.Float64Histogram
	retryer                   *Retryer
}

func NewAutoUpgrader(repo database.Repo, log logrus.FieldLogger, client ClusterManager, meter metric.Meter) *AutoUpgrader {
	autoUpgradesScheduled, _ := meter.Int64Counter(
		"auto_upgrades_scheduled",
		metric.WithDescription("Number of automatic cluster upgrades scheduled"),
	)
	autoUpgradeProcessingTime, _ := meter.Float64Histogram(
		"auto_upgrade_processing_time_seconds",
		metric.WithDescription("Time taken to process automatic upgrades"),
		metric.WithUnit("s"),
	)
	gkeAPICalls, _ := meter.Int64Counter(
		"auto_upgrader_gke_api_calls",
		metric.WithDescription("Number of GKE API calls made by auto upgrader"),
	)
	gkeAPIErrors, _ := meter.Int64Counter(
		"auto_upgrader_gke_api_errors",
		metric.WithDescription("Number of GKE API errors encountered by auto upgrader"),
	)
	retryAttempts, _ := meter.Int64Counter(
		"auto_upgrader_retry_attempts",
		metric.WithDescription("Number of retry attempts made by auto upgrader"),
	)

	retryer := NewRetryer(log, gkeAPICalls, gkeAPIErrors, retryAttempts, DefaultRetryConfig())

	return &AutoUpgrader{
		repo:                      repo,
		log:                       log,
		client:                    client,
		autoUpgradesScheduled:     autoUpgradesScheduled,
		autoUpgradeProcessingTime: autoUpgradeProcessingTime,
		retryer:                   retryer,
	}
}

func (c *AutoUpgrader) Run(ctx context.Context) error {
	startTime := time.Now()
	c.log.WithFields(logrus.Fields{
		"component":   "auto_upgrader",
		"time_window": "9-16",
	}).Debug("starting auto-upgrader run")

	defer func() {
		duration := time.Since(startTime).Seconds()
		c.autoUpgradeProcessingTime.Record(ctx, duration)
		c.log.WithFields(logrus.Fields{
			"component":        "auto_upgrader",
			"duration_seconds": duration,
		}).Debug("auto-upgrader run completed")
	}()

	if !c.client.IsTimeInRange(9, 16) {
		c.log.WithFields(logrus.Fields{
			"component":    "auto_upgrader",
			"time_window":  "9-16",
			"current_time": time.Now().Format("15:04"),
		}).Info("outside configured time window for auto-upgrade")
		return nil
	}

	envs, err := c.repo.EnvironmentsGetByAutoUpgrade(ctx)
	if err != nil {
		c.log.WithFields(logrus.Fields{
			"component": "auto_upgrader",
			"operation": "get_environments",
		}).WithError(err).Error("failed to retrieve environments with auto-upgrade enabled")
		return err
	}

	c.log.WithFields(logrus.Fields{
		"component":         "auto_upgrader",
		"environment_count": len(envs),
	}).Debug("processing environments for auto-upgrade evaluation")

	processedCount := 0
	scheduledCount := 0

	for _, env := range envs {
		envLogger := c.createEnvironmentLogger(env, nil)

		if processed, scheduled := c.processEnvironment(ctx, env, envLogger); processed {
			processedCount++
			if scheduled {
				scheduledCount++
			}
		}
	}

	c.log.WithFields(logrus.Fields{
		"component":              "auto_upgrader",
		"environments_processed": processedCount,
		"upgrades_scheduled":     scheduledCount,
		"total_environments":     len(envs),
	}).Debug("auto-upgrader run summary")

	return nil
}

// createEnvironmentLogger creates a logger with environment context
func (c *AutoUpgrader) createEnvironmentLogger(env *model.Environment, tenant *model.Tenant) logrus.FieldLogger {
	fields := logrus.Fields{
		"component":      "auto_upgrader",
		"environment_id": env.ID,
		"environment":    env.Name,
	}

	if tenant != nil {
		fields["tenant_id"] = tenant.ID
		fields["tenant"] = tenant.Name
	}

	return c.log.WithFields(fields)
}

// processEnvironment handles the upgrade evaluation for a single environment
func (c *AutoUpgrader) processEnvironment(ctx context.Context, env *model.Environment, envLogger logrus.FieldLogger) (processed, scheduled bool) {
	projectID, err := c.getProjectID(ctx, env.ID)
	if err != nil {
		envLogger.WithError(err).Error("failed to retrieve project ID from environment configuration")
		return false, false
	}

	tenant, err := c.repo.TenantGet(ctx, env.TenantID)
	if err != nil {
		envLogger.WithError(err).Error("failed to retrieve tenant information")
		return false, false
	}

	// Update logger with tenant context
	envLogger = c.createEnvironmentLogger(env, tenant)

	envLogger.WithFields(logrus.Fields{
		"project_id": projectID,
	}).Debug("evaluating environment for automatic upgrades")

	controlPlaneVer, err := c.getCurrentControlPlaneVersionWithRetry(ctx, projectID, env)
	if err != nil {
		envLogger.WithFields(logrus.Fields{
			"project_id": projectID,
			"operation":  "get_control_plane_version",
		}).WithError(err).Error("failed to retrieve current cluster control plane version")
		return true, false
	}

	channel, err := c.getReleaseChannelWithRetry(ctx, projectID, env)
	if err != nil {
		envLogger.WithFields(logrus.Fields{
			"project_id":            projectID,
			"control_plane_version": controlPlaneVer,
			"operation":             "get_release_channel",
		}).WithError(err).Error("failed to retrieve cluster release channel")
		return true, false
	}

	availableVersions, err := c.getAvailableVersionsWithRetry(ctx, projectID, env, channel)
	if err != nil {
		envLogger.WithFields(logrus.Fields{
			"project_id":            projectID,
			"control_plane_version": controlPlaneVer,
			"release_channel":       channel,
			"operation":             "get_available_versions",
		}).WithError(err).Error("failed to retrieve available cluster versions")
		return true, false
	}

	envLogger.WithFields(logrus.Fields{
		"control_plane_version": controlPlaneVer,
		"release_channel":       channel,
		"available_versions":    len(availableVersions),
	}).Debug("retrieved cluster version information")

	return c.evaluateAndScheduleUpgrades(ctx, env, envLogger, controlPlaneVer, availableVersions)
}

// evaluateAndScheduleUpgrades checks for newer patch versions and schedules upgrades
func (c *AutoUpgrader) evaluateAndScheduleUpgrades(ctx context.Context, env *model.Environment, envLogger logrus.FieldLogger, controlPlaneVer string, availableVersions []string) (processed, scheduled bool) {
	tenant, _ := c.repo.TenantGet(ctx, env.TenantID) // Already retrieved in parent, but needed for metrics

	for _, version := range availableVersions {
		if c.IsNewerPatchRelease(controlPlaneVer, version) {
			envLogger.WithFields(logrus.Fields{
				"current_version": controlPlaneVer,
				"target_version":  version,
				"upgrade_type":    "patch",
			}).Info("newer patch version detected, evaluating for upgrade scheduling")

			// Check if upgrade already in progress
			status, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
			if err != nil {
				envLogger.WithFields(logrus.Fields{
					"target_version": version,
					"operation":      "check_existing_upgrade",
				}).WithError(err).Error("failed to check existing cluster upgrade status")
				return true, false
			}

			if status != nil {
				envLogger.WithFields(logrus.Fields{
					"current_version":  controlPlaneVer,
					"target_version":   version,
					"existing_upgrade": status.ID,
					"upgrade_status":   status.UpgradeStatus,
				}).Info("cluster upgrade already in progress, skipping automatic scheduling")
				return true, false
			}

			// Schedule the upgrade
			upgrade, err := c.repo.CreateClusterUpgrade(ctx, env.TenantID, env.ID, version)
			if err != nil {
				envLogger.WithFields(logrus.Fields{
					"current_version": controlPlaneVer,
					"target_version":  version,
					"operation":       "create_upgrade",
				}).WithError(err).Error("failed to schedule automatic cluster upgrade")
				return true, false
			}

			// Record successful scheduling
			c.autoUpgradesScheduled.Add(ctx, 1, metric.WithAttributes(
				attribute.String("environment", env.Name),
				attribute.String("tenant", tenant.Name),
				attribute.String("current_version", controlPlaneVer),
				attribute.String("target_version", version),
			))

			envLogger.WithFields(logrus.Fields{
				"current_version": controlPlaneVer,
				"target_version":  version,
				"upgrade_id":      upgrade.ID,
				"upgrade_type":    "patch",
			}).Info("automatic cluster upgrade scheduled successfully")

			return true, true
		}
	}

	envLogger.WithFields(logrus.Fields{
		"control_plane_version": controlPlaneVer,
		"evaluated_versions":    len(availableVersions),
	}).Debug("no newer patch versions found for automatic upgrade")

	return true, false
}

func (c *AutoUpgrader) IsNewerPatchRelease(current, new string) bool {
	v1, err := version.NewVersion(current)
	if err != nil {
		c.log.WithFields(logrus.Fields{
			"component":       "auto_upgrader",
			"operation":       "version_comparison",
			"current_version": current,
			"error_type":      "parse_current_version",
		}).WithError(err).Error("failed to parse current cluster version - this indicates a data quality issue")
		return false
	}

	v2, err := version.NewVersion(new)
	if err != nil {
		c.log.WithFields(logrus.Fields{
			"component":       "auto_upgrader",
			"operation":       "version_comparison",
			"current_version": current,
			"new_version":     new,
			"error_type":      "parse_new_version",
		}).WithError(err).Error("failed to parse candidate version from GKE - this may indicate an API issue")
		return false
	}

	// Split versions to extract major.minor.patch
	v1Segments := strings.Split(v1.String(), "-")[0]
	v2Segments := strings.Split(v2.String(), "-")[0]

	v1Parts := strings.Split(v1Segments, ".")
	v2Parts := strings.Split(v2Segments, ".")

	if len(v1Parts) < 3 || len(v2Parts) < 3 {
		c.log.WithFields(logrus.Fields{
			"component":       "auto_upgrader",
			"operation":       "version_comparison",
			"current_version": current,
			"new_version":     new,
			"current_parts":   len(v1Parts),
			"new_parts":       len(v2Parts),
			"error_type":      "invalid_version_format",
		}).Error("version format validation failed - versions must include major.minor.patch components")
		return false
	}

	// Compare major and minor versions - only allow patch upgrades
	if v1Parts[0] == v2Parts[0] && v1Parts[1] == v2Parts[1] {
		isNewer := v2.GreaterThan(v1)
		c.log.WithFields(logrus.Fields{
			"component":       "auto_upgrader",
			"operation":       "version_comparison",
			"current_version": current,
			"new_version":     new,
			"major_version":   v1Parts[0],
			"minor_version":   v1Parts[1],
			"is_newer_patch":  isNewer,
		}).Debug("completed patch version comparison")
		return isNewer
	}

	c.log.WithFields(logrus.Fields{
		"component":         "auto_upgrader",
		"operation":         "version_comparison",
		"current_version":   current,
		"new_version":       new,
		"current_major":     v1Parts[0],
		"current_minor":     v1Parts[1],
		"new_major":         v2Parts[0],
		"new_minor":         v2Parts[1],
		"comparison_result": "different_major_minor",
	}).Debug("skipping version - not a patch release (different major/minor version)")

	return false
}

func (c *AutoUpgrader) getProjectID(ctx context.Context, environmentID uuid.UUID) (string, error) {
	projectID, err := c.repo.EnvironmentValueGet(ctx, environmentID, "project_id", false)
	if err != nil {
		c.log.WithFields(logrus.Fields{
			"component":      "auto_upgrader",
			"operation":      "get_project_id",
			"environment_id": environmentID,
			"key":            "project_id",
		}).WithError(err).Debug("failed to retrieve project_id from environment configuration")
		return "", err
	}

	id := ""
	if err := json.Unmarshal(projectID.Value, &id); err != nil {
		c.log.WithFields(logrus.Fields{
			"component":      "auto_upgrader",
			"operation":      "get_project_id",
			"environment_id": environmentID,
			"key":            "project_id",
			"raw_value":      string(projectID.Value),
		}).WithError(err).Warn("failed to parse project_id JSON value from environment configuration")
		return "", err
	}

	if id == "" {
		c.log.WithFields(logrus.Fields{
			"component":      "auto_upgrader",
			"operation":      "get_project_id",
			"environment_id": environmentID,
		}).Warn("project_id is empty in environment configuration")
		return "", errors.New("project_id is empty")
	}

	return id, nil
}

// Helper methods with retry logic for GKE API calls
func (c *AutoUpgrader) getCurrentControlPlaneVersionWithRetry(ctx context.Context, projectID string, env *model.Environment) (string, error) {
	var controlPlaneVer string
	err := c.retryer.WithBackoff(ctx, "get_current_control_plane_version", func() error {
		var retryErr error
		controlPlaneVer, retryErr = c.client.GetCurrentControlPlaneVersion(ctx, projectID, env)
		return retryErr
	})
	return controlPlaneVer, err
}

func (c *AutoUpgrader) getReleaseChannelWithRetry(ctx context.Context, projectID string, env *model.Environment) (string, error) {
	var channel string
	err := c.retryer.WithBackoff(ctx, "get_release_channel", func() error {
		var retryErr error
		channel, retryErr = c.client.GetReleaseChannel(ctx, projectID, env)
		return retryErr
	})
	return channel, err
}

func (c *AutoUpgrader) getAvailableVersionsWithRetry(ctx context.Context, projectID string, env *model.Environment, channel string) ([]string, error) {
	var availableVersions []string
	err := c.retryer.WithBackoff(ctx, "get_available_versions", func() error {
		var retryErr error
		availableVersions, retryErr = c.client.GetAvailableVersions(ctx, projectID, env, channel)
		return retryErr
	})
	return availableVersions, err
}
