package naisd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nais/fasit/internal/message"
)

type HealthReporter struct {
	pub    StatusPublisher
	tenant string
	env    string
}

func NewHealthReporter(tenant, env string, pub StatusPublisher) *HealthReporter {
	return &HealthReporter{
		pub:    pub,
		tenant: tenant,
		env:    env,
	}
}

func (s *HealthReporter) Run(ctx context.Context) error {
	hr := message.Health{
		ReportedAt: time.Now(),
	}

	hrb, err := json.Marshal(hr)
	if err != nil {
		return fmt.Errorf("json.Marshal: %w", err)
	}

	return s.pub.Publish(ctx, message.Status{
		Tenant:      s.tenant,
		Environment: s.env,
		Type:        message.StatusTypeHealth,
		Data:        hrb,
	})
}
