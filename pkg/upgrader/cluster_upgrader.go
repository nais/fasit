package upgrader

import (
	"context"
	"fmt"

	"github.com/google/uuid"
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
		envs, err := c.repo.EnvironmentsGet(ctx, tenant.ID)
		if err != nil {
			return err
		}
		for _, env := range envs {
			// TODO: Check if cluster is upgradable
			// er det kjørende operasjoner i dben
			// er det kjørende operasjoner på clusteret?
			// er master versjonen den samme som den nyeste tilgjengelige?
			// er nodepool versjonen den samme som master versjonen?

			// sjekk operations i clusteret -> upgrade master -> kall til google -> opprett operasjon i db
			// fant running op i db -> kall til google (getOperation) -> oppdater operasjon i db
			// done -> oppgrader nodepool

			c.log.Infof("Upgrading clusters for tenant %s, env %s", tenant.Name, env.Name)
			projectId, err := getProjectId(ctx, c, env.ID)

			runningOperations, err := c.repo.GetRunningClusterOperations(ctx, tenant.ID, env.ID)
			if err != nil {
				return err
			}

			if len(runningOperations) == 1 {
				runningOperation := runningOperations[0]
				op, err := c.client.GetOperation(ctx, projectId, runningOperation.OperationID)
				if err != nil {
					return err
				}

				c.repo.CreateOrUpdateClusterOperation(ctx, tenant.ID, env.ID, op)
				continue

			} else if len(runningOperations) > 1 {
				// TODO: metric / alert på denne
				return fmt.Errorf("found %d running operations for tenant %s, env %s. should be only one", len(runningOperations), tenant.Name, env.Name)
			}

			availableVersions, err := c.client.GetAvailableVersions(ctx, projectId, env.Name, "STABLE")
			if err != nil {
				return err
			}

			fmt.Println("Available versions", availableVersions)

			/*

				master_version, err := c.client.GetCurrentMasterVersion(ctx, pid_string, env.Name)
				if err != nil {
					return err
				}

				_, err := c.client.GetAvailableVersions(ctx, pid_string, env.Name, "STABLE")
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
				}*/
		}

	}
	return nil
}

// TODO: thomas er dette ok?
func getProjectId(ctx context.Context, c *ClusterUpgrader, environmentId uuid.UUID) (string, error) {
	projectId, err := c.repo.EnvironmentValueGet(ctx, environmentId, "project_id", false)
	if err != nil {
		return "", err
	}

	pid, err := projectId.Value.MarshalJSON()
	if err != nil {
		return "", err
	}

	pid_string := string(pid[1 : len(pid)-1])
	return pid_string, nil
}
