package fake

import "context"

type Upgrader struct{}

func NewUpgrader() *Upgrader {
	return &Upgrader{}
}

func (c *Upgrader) GetReleaseChannel(ctx context.Context, projectId, clusterName string) (string, error) {
	return "STABLE", nil
}
func (c *Upgrader) GetCurrentMasterVersion(ctx context.Context, projectId, clusterName string) (string, error) {
	return "1.15.12-gke.2", nil
}
func (c *Upgrader) GetAvailableVersions(ctx context.Context, projectId, clusterName, releaseChannel string) ([]string, error) {
	return []string{"1.15.12-gke.2", "1.15.12-gke.3", "1.15.12-gke.4"}, nil
}
