package integration_test

import (
	"context"
	"encoding/json"

	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

type MockPublisher struct {
	deployInstruction []message.DeployInstruction
	ch                chan message.Status
	tenant            string
	environment       string
}

func (m *MockPublisher) Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error {
	for s := range m.ch {
		if err := f(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	m.deployInstruction = append(m.deployInstruction, msg)
	return nil
}

func (m *MockPublisher) SendStatus(status model.RolloutStatus) {
	for _, msg := range m.deployInstruction {
		msgHelm := &message.Helm{
			DIID:          msg.ID,
			RolloutStatus: status,
			ConfigHash:    msg.ConfigHash,
			Log:           "",
		}

		b, err := json.Marshal(msgHelm)
		if err != nil {
			panic(err)
		}

		m.ch <- message.Status{
			Type:        message.StatusTypeHelm,
			Data:        b,
			Tenant:      m.tenant,
			Environment: m.environment,
		}
	}
}

func (m *MockPublisher) Stop() {}

func (m *MockPublisher) DeployInstructions() []message.DeployInstruction {
	return m.deployInstruction
}
