package workers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

func TestReceiver(t *testing.T) {
	uid := uuid.New()
	tests := map[string]struct {
		envID           uuid.UUID
		statuses        []message.Status
		expectedUpdates int
	}{
		"empty": {
			envID:           uid,
			statuses:        []message.Status{},
			expectedUpdates: 0,
		},
		"one": {
			envID: uid,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelm,
					Partner:     "partner",
					Environment: "env",
					Data:        []byte(`{"name":"test","namespace":"test","status":"deployed","chart":"test","version":"1.0.0","appVersion":"1.0.0","values":{}}`),
				},
			},
			expectedUpdates: 1,
		},
		"missing partner": {
			envID: uuid.Nil,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelm,
					Partner:     "partner",
					Environment: "env",
					Data:        []byte(`{"name":"test","namespace":"test","status":"deployed","chart":"test","version":"1.0.0","appVersion":"1.0.0","values":{}}`),
				},
			},
			expectedUpdates: 0,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage := &mockStorage{envID: tc.envID}
			rec := NewReceiver(
				&mockReceiverClient{messages: tc.statuses},
				storage,
				logrus.NewEntry(logrus.StandardLogger()),
			)

			rec.Run(context.Background())
			if storage.statusCreateOrUpdate != tc.expectedUpdates {
				t.Errorf("expected %d status messages to be handled, got %d", len(tc.statuses), storage.statusCreateOrUpdate)
			}
		})
	}
}

type mockReceiverClient struct {
	messages []message.Status
}

func (m *mockReceiverClient) Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error {
	for _, msg := range m.messages {
		if err := f(ctx, msg); err != nil {
			return err
		}
	}
	return nil
}

type mockStorage struct {
	envID                uuid.UUID
	statusCreateOrUpdate int
}

func (m *mockStorage) EnvironmentIDByNames(ctx context.Context, partnerName string, environmentName string) (uuid.UUID, error) {
	if m.envID == uuid.Nil {
		return uuid.Nil, sql.ErrNoRows
	}
	return m.envID, nil
}

func (m *mockStorage) StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Helm) error {
	m.statusCreateOrUpdate++
	return nil
}
