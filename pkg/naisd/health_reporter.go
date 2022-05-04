package naisd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

type HealthReporter struct {
	pub    StatusPublisher
	tenant string
	env    string
	kind   model.EnvironmentKind
}

func NewHealthReporter(tenant, env string, kind model.EnvironmentKind, pub StatusPublisher) *HealthReporter {
	return &HealthReporter{
		pub:    pub,
		tenant: tenant,
		env:    env,
		kind:   kind,
	}
}

func (s *HealthReporter) Run(ctx context.Context) error {
	hr := message.Health{
		Kind:       s.kind,
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
