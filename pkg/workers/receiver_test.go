package workers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

func TestReceiver(t *testing.T) {
	uid := uuid.New()
	tests := map[string]struct {
		envID                          uuid.UUID
		statuses                       []message.Status
		numStatusCreateOrUpdate        int
		numReleaseStatusCreateOrUpdate int
		numHealthStatusCreateOrUpdate  int
		numKubernetesNodeSync          int
	}{
		"empty": {
			envID:    uid,
			statuses: []message.Status{},
		},
		"helm one": {
			envID: uid,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelm,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{"name":"test","namespace":"test","status":"deployed","chart":"test","version":"1.0.0","appVersion":"1.0.0","values":{}}`),
				},
			},
			numStatusCreateOrUpdate: 1,
		},
		"helm missing tenant": {
			envID: uuid.Nil,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelm,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{"name":"test","namespace":"test","status":"deployed","chart":"test","version":"1.0.0","appVersion":"1.0.0","values":{}}`),
				},
			},
			numStatusCreateOrUpdate: 1,
		},
		"helm releases": {
			envID: uid,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelmReleases,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{"releases":[{"name":"test","namespace":"test","status":"deployed","chart":"test","version":"1.0.0","appVersion":"1.0.0","values":{}}]}`),
				},
			},
			numReleaseStatusCreateOrUpdate: 1,
		},
		"health status": {
			envID: uid,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHealth,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{"reportedAt": "2020-01-01T00:00:00Z"}`),
				},
			},
			numHealthStatusCreateOrUpdate: 1,
		},
		"kubernetes nodes": {
			envID: uid,
			statuses: []message.Status{
				{
					Type:        message.StatusKubernetesNodes,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{}`),
				},
			},
			numKubernetesNodeSync: 1,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			repo := mocks.NewRepo(t)
			repo.On("EnvironmentIDByNames", mock.Anything, "tenant", "env").Return(tc.envID, nil).Maybe().Times(len(tc.statuses))
			if tc.numStatusCreateOrUpdate > 0 {
				repo.On("StatusCreateOrUpdate", mock.Anything, tc.envID, mock.Anything).Return(nil).Times(tc.numStatusCreateOrUpdate)
			}
			if tc.numReleaseStatusCreateOrUpdate > 0 {
				repo.On("ReleaseStatusCreateOrUpdate", mock.Anything, tc.envID, mock.Anything).Return(nil).Times(tc.numReleaseStatusCreateOrUpdate)
			}
			if tc.numHealthStatusCreateOrUpdate > 0 {
				repo.On("HealthStatusCreateOrUpdate", mock.Anything, tc.envID, mock.Anything).Return(nil).Times(tc.numHealthStatusCreateOrUpdate)
			}
			if tc.numKubernetesNodeSync > 0 {
				repo.On("KubernetesNodeSync", mock.Anything, tc.envID, mock.Anything).Return(nil).Times(tc.numKubernetesNodeSync)
			}

			rec := NewReceiver(
				&mockReceiverClient{messages: tc.statuses},
				repo,
				func(ctx context.Context, id uuid.UUID, status model.RolloutStatus) error { return nil },
				logrus.NewEntry(logrus.StandardLogger()),
			)

			rec.Run(context.Background())
			// if storage.statusCreateOrUpdate != tc.expectedUpdates {
			// 	t.Errorf("expected %d status messages to be handled, got %d", len(tc.statuses), storage.statusCreateOrUpdate)
			// }
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
