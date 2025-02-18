package naisd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nais/fasit/internal/message"
	"helm.sh/helm/v3/pkg/release"
)

type HelmClient interface {
	List() ([]*release.Release, error)
}

type StatusReporter struct {
	client HelmClient
	pub    StatusPublisher
	tenant string
	env    string
}

func NewStatusReporter(tenant, env string, client HelmClient, pub StatusPublisher) *StatusReporter {
	return &StatusReporter{
		client: client,
		pub:    pub,
		tenant: tenant,
		env:    env,
	}
}

func (s *StatusReporter) Run(ctx context.Context) error {
	releases, err := s.client.List()
	if err != nil {
		return fmt.Errorf("client.List: %w", err)
	}

	hr := message.HelmRelease{
		Created: time.Now(),
	}

	for _, r := range releases {
		hr.Releases = append(hr.Releases, message.Release{
			Name:         r.Name,
			Version:      r.Chart.Metadata.Version,
			Status:       r.Info.Status.String(),
			Revision:     r.Version,
			LastDeployed: r.Info.LastDeployed.Time,
		})
	}

	hrb, err := json.Marshal(hr)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	return s.pub.Publish(ctx, message.Status{
		Tenant:      s.tenant,
		Environment: s.env,
		Type:        message.StatusTypeHelmReleases,
		Data:        hrb,
	})
}

func (s *StatusReporter) Trigger() {
	if err := s.Run(context.Background()); err != nil {
		fmt.Println("Error reporting status: ", err)
	}
}
