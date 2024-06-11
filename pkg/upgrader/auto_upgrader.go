package upgrader

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/go-version"
	"github.com/nais/fasit/pkg/database"
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
	c.log.Info("Starting auto-upgrader")
	defer c.log.Info("Auto-upgrader stopped")

	envs, err := c.repo.EnvironmentsGetByAutoUpgrade(ctx)
	if err != nil {
		c.log.WithError(err).Error("Failed to get environments")
		return err
	}

	for _, env := range envs {
		projectId, err := c.getProjectId(ctx, env.ID)
		if err != nil {
			c.log.WithError(err).Error("Failed to get project id")
			continue
		}
		tenant, err := c.repo.TenantGet(ctx, env.TenantID)
		if err != nil {
			c.log.WithError(err).Error("Failed to get tenant")
			continue
		}
		masterVer, err := c.client.GetCurrentMasterVersion(ctx, projectId, env)
		if err != nil {
			c.log.WithError(err).Error("Failed to get current master version")
			continue
		}
		channel, err := c.client.GetReleaseChannel(ctx, projectId, env)
		if err != nil {
			c.log.WithError(err).Error("Failed to get release channel")
			continue
		}
		availableVersions, err := c.client.GetAvailableVersions(ctx, projectId, env, channel)
		if err != nil {
			c.log.WithError(err).Error("Failed to get available versions")
			continue
		}

		for _, version := range availableVersions {
			if c.IsNewerPatchRelease(masterVer, version) {
				c.log.Infof("Upgrading %s:%s to version %s", tenant.Name, env.Name, version)

				status, err := c.repo.ClusterUpgradeGet(ctx, env.TenantID, env.ID)
				if err != nil {
					c.log.WithError(err).Error("Failed to get cluster upgrade status")
					break
				}
				if status != nil {
					c.log.Infof("Cluster upgrade already in progress for %s:%s", tenant.Name, env.Name)
					break
				}

				upgrade, err := c.repo.CreateClusterUpgrade(ctx, env.TenantID, env.ID, version)
				if err != nil {
					c.log.WithError(err).Error("Failed to create cluster upgrade")
					break
				}
				c.log.Infof("Cluster upgrade created for %s:%s: %v", tenant.Name, env.Name, upgrade)
			}
		}
	}

	return nil
}

func (c *AutoUpgrader) IsNewerPatchRelease(current, new string) bool {
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

func (c *AutoUpgrader) getProjectId(ctx context.Context, environmentId uuid.UUID) (string, error) {
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
