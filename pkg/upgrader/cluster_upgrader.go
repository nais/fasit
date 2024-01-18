package upgrader

import (
	"context"
	"fmt"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/graph"

	"github.com/sirupsen/logrus"
)

type ClusterUpgrader struct {
	log    logrus.FieldLogger
	repo   database.Repo
	client graph.Upgrader
}

func NewClusterUpgrader(repo database.Repo, log logrus.FieldLogger, upgrader graph.Upgrader) *ClusterUpgrader {
	return &ClusterUpgrader{
		log:    log,
		repo:   repo,
		client: upgrader,
	}
}

func (c *ClusterUpgrader) Run(ctx context.Context) error {
	c.log.Info("Running cluster upgrader")
	tenants, err := c.repo.TenantsGet(ctx)
	if err != nil {
		return err
	}
	for _, tenant := range tenants {
		c.log.Infof("Upgrading clusters for tenant %s", tenant.Name)
		envs, err := c.repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			return err
		}
		for _, env := range envs {
			c.log.Infof("Upgrading %s", env.Name)

			projectId, err := c.repo.EnvironmentValueGet(ctx, env.ID, "project_id", false)
			if err != nil {
				return err
			}

			pid, err := projectId.Value.MarshalJSON()
			if err != nil {
				return err
			}

			pid_string := string(pid[1 : len(pid)-1])
			fmt.Println("projectId", pid_string)

			master_version, err := c.client.GetCurrentMasterVersion(ctx, pid_string, env.Name)
			if err != nil {
				return err
			}

			ops, err := c.client.GetRunningOperations(ctx, pid_string, env.Name)
			if err != nil {
				return err
			}

			for _, op := range ops {
				fmt.Println("Operation", op.Name)
				u, err := c.repo.ClusterOperationCreateOrUpdate(ctx, tenant.ID, env.ID, master_version, op)
				if err != nil {
					return err
				}
				fmt.Println("ClusterOperationCreateOrUpdate", u)
			}
		}

	}
	return nil
}
