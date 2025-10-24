package upgrader

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/nais/fasit/internal/database"
	"github.com/sirupsen/logrus"
)

type AutoUpgrader struct {
	repo   database.Repo
	log    logrus.FieldLogger
	client Upgrader
}

func NewAutoUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader Upgrader) *AutoUpgrader {
	return &AutoUpgrader{
		repo:   repo,
		log:    log,
		client: upgrader,
	}
}

func (c *AutoUpgrader) Run(ctx context.Context) error {
	c.log.Debug("starting auto-upgrader")
	defer c.log.Debug("auto-upgrader stopped")

	if !c.client.IsTimeInRange(9, 16) {
		c.log.Debug("not in time range for auto-upgrade")
		return nil
	}

	envs, err := c.repo.EnvironmentsGetByAutoUpgrade(ctx)
	if err != nil {
		c.log.WithError(err).Error("failed to get environments")
		return err
	}

	for _, env := range envs {
		projectID, err := c.getProjectID(ctx, env.ID)
		if err != nil {
			c.log.WithFields(logrus.Fields{"environment": env.Name}).WithError(err).Error("failed to get project id")
			continue
		}
		tenant, err := c.repo.TenantGet(ctx, env.TenantID)
		if err != nil {
			c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).WithError(err).Error("failed to get tenant")
			continue
		}
		masterVer, err := c.client.GetCurrentMasterVersion(ctx, projectID, env)
		if err != nil {
			c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).WithError(err).Error("failed to get current master version")
			continue
		}
		channel, err := c.client.GetReleaseChannel(ctx, projectID, env)
		if err != nil {
			c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).WithError(err).Error("failed to get release channel")
			continue
		}
		availableVersions, err := c.client.GetAvailableVersions(ctx, projectID, env, channel)
		if err != nil {
			c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).WithError(err).Error("failed to get available versions")
			continue
		}

		for _, version := range availableVersions {
			if c.IsNewerPatchRelease(masterVer, version) {
				c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Infof("upgrading to version %s", version)

				status, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
				if err != nil {
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).WithError(err).Error("failed to get cluster upgrade status")
					break
				}
				if status != nil {
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Info("cluster upgrade already in progress")
					break
				}

				upgrade, err := c.repo.CreateClusterUpgrade(ctx, env.TenantID, env.ID, version)
				if err != nil {
					c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).WithError(err).Error("Failed to create cluster upgrade")
					break
				}
				c.log.WithFields(logrus.Fields{"tenant": tenant.Name, "environment": env.Name}).Debugf("cluster upgrade created for: %s", upgrade.ID)
			}
		}
	}

	return nil
}

func (c *AutoUpgrader) IsNewerPatchRelease(current, new string) bool {
	v1, err := version.NewVersion(current)
	if err != nil {
		c.log.WithError(err).Fatalf("error parsing version1: %s", err)
	}

	v2, err := version.NewVersion(new)
	if err != nil {
		c.log.WithError(err).Fatalf("error parsing version2: %s", err)
	}

	// Split versions to extract major.minor.patch
	v1Segments := strings.Split(v1.String(), "-")[0]
	v2Segments := strings.Split(v2.String(), "-")[0]

	v1Parts := strings.Split(v1Segments, ".")
	v2Parts := strings.Split(v2Segments, ".")

	if len(v1Parts) < 3 || len(v2Parts) < 3 {
		c.log.Fatalf("invalid version format, must include major.minor.patch")
	}

	// Compare major and minor versions
	if v1Parts[0] == v2Parts[0] && v1Parts[1] == v2Parts[1] {
		return v2.GreaterThan(v1)
	}

	return false
}

func (c *AutoUpgrader) getProjectID(ctx context.Context, environmentID uuid.UUID) (string, error) {
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
