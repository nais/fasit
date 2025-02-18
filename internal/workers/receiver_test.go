package workers

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/mocks"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

func TestReceiver(t *testing.T) {
	uid := uuid.New()
	diid := uuid.New()
	tests := map[string]struct {
		envID                          uuid.UUID
		deployInstructionID            uuid.UUID
		helmStatus                     bool
		statuses                       []message.Status
		numStatusCreateOrUpdate        int
		numReleaseStatusCreateOrUpdate int
		numHealthStatusCreateOrUpdate  int
		numKubernetesNodeSync          int
	}{
		"empty": {
			envID:               uid,
			deployInstructionID: diid,
			statuses:            []message.Status{},
		},
		"helm one": {
			envID:               uid,
			deployInstructionID: diid,
			helmStatus:          true,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelm,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{"name":"test","rolloutStatus":"deployed","version":"1.0.0","DIID":"` + diid.String() + `"}`),
				},
			},
			numStatusCreateOrUpdate: 1,
		},
		"helm missing tenant": {
			envID:               uuid.Nil,
			deployInstructionID: diid,
			helmStatus:          true,
			statuses: []message.Status{
				{
					Type:        message.StatusTypeHelm,
					Tenant:      "tenant",
					Environment: "env",
					Data:        []byte(`{"name":"test","rolloutStatus":"deployed","version":"1.0.0","DIID":"` + diid.String() + `"}`),
				},
			},
			numStatusCreateOrUpdate: 1,
		},
		"helm releases": {
			envID:               uid,
			deployInstructionID: diid,
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
			envID:               uid,
			deployInstructionID: diid,
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
			envID:               uid,
			deployInstructionID: diid,
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

			repo.On("DeployInstructionGet", mock.Anything, tc.deployInstructionID).Return(&model.DeployInstruction{ID: tc.deployInstructionID, EnvironmentID: tc.envID}, nil).Maybe()
			repo.On("EnvironmentGet", mock.Anything, tc.envID).Return(&model.Environment{ID: tc.envID}, nil).Maybe()

			if !tc.helmStatus {
				repo.On("EnvironmentIDByNames", mock.Anything, "tenant", "env").Return(tc.envID, nil).Maybe().Times(len(tc.statuses))
			}

			if tc.numStatusCreateOrUpdate > 0 {
				repo.On("DeployInstructionUpdateStatus", mock.Anything, mock.Anything, model.RolloutStatusDeployed).Return(nil).Times(tc.numStatusCreateOrUpdate)
			}
			if tc.numReleaseStatusCreateOrUpdate > 0 {
				repo.On("ReleaseStatusDeleteByEnvironmentID", mock.Anything, tc.envID).Return(nil).Times(tc.numReleaseStatusCreateOrUpdate)
				repo.On("ReleaseStatusCreateOrUpdate", mock.Anything, tc.envID, mock.Anything).Return(nil).Times(tc.numReleaseStatusCreateOrUpdate)
				repo.On("TxFunc", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
					args.Get(1).(database.TXFunc)(repo)
				})
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
				logrus.NewEntry(logrus.StandardLogger()),
				fake.NewFakeSlackClient(),
				"test",
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
