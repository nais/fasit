package workers

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/sirupsen/logrus"
)

func TestReceiver_ReleaseStatus(t *testing.T) {
	envID := uuid.New()

	var deletedEnvID uuid.UUID
	var createdReleases []message.Release

	store := &fakeStore{
		environmentIDByNamesFunc: func(_ context.Context, tenant, env string) (uuid.UUID, error) {
			if tenant != "t1" || env != "e1" {
				t.Fatalf("EnvironmentIDByNames(%q, %q)", tenant, env)
			}
			return envID, nil
		},
		txFuncFunc: func(ctx context.Context, fn database.TXFunc) error {
			// The callback receives a database.Repo. We provide a minimal
			// fake that only implements the release-status methods called
			// inside releaseStatus's transaction.
			txRepo := &fakeTxRepo{
				deleteFunc: func(_ context.Context, id uuid.UUID) error {
					deletedEnvID = id
					return nil
				},
				createFunc: func(_ context.Context, id uuid.UUID, r *message.Release) error {
					if id != envID {
						t.Fatalf("ReleaseStatusCreateOrUpdate env=%v, want %v", id, envID)
					}
					createdReleases = append(createdReleases, *r)
					return nil
				},
			}
			return fn(txRepo)
		},
	}

	data, _ := json.Marshal(message.HelmRelease{
		Releases: []message.Release{{Name: "app1", Version: "1.0.0", Status: "deployed"}},
	})

	rec := NewReceiver(
		&fakeClient{messages: []message.Status{
			{Type: message.StatusTypeHelmReleases, Tenant: "t1", Environment: "e1", Data: data},
		}},
		store,
		logrus.NewEntry(logrus.New()),
		fake.NewFakeSlackClient(),
		"test",
	)
	rec.Run(context.Background())

	if deletedEnvID != envID {
		t.Errorf("ReleaseStatusDeleteByEnvironmentID(%v), want %v", deletedEnvID, envID)
	}
	if len(createdReleases) != 1 {
		t.Fatalf("got %d creates, want 1", len(createdReleases))
	}
	if createdReleases[0].Name != "app1" {
		t.Errorf("release.Name = %q, want %q", createdReleases[0].Name, "app1")
	}
}

func TestReceiver_UnknownType(t *testing.T) {
	rec := NewReceiver(
		&fakeClient{
			messages: []message.Status{{Type: 99, Data: []byte(`{}`)}},
		},
		&fakeStore{},
		logrus.NewEntry(logrus.New()),
		fake.NewFakeSlackClient(),
		"test",
	)
	rec.Run(context.Background())
}

// --- fakes ---

type fakeClient struct {
	messages []message.Status
}

func (f *fakeClient) Receive(ctx context.Context, fn func(context.Context, message.Status) error) error {
	for _, m := range f.messages {
		if err := fn(ctx, m); err != nil {
			return err
		}
	}
	return nil
}

// fakeStore implements ReceiverStore.
type fakeStore struct {
	environmentIDByNamesFunc func(context.Context, string, string) (uuid.UUID, error)
	txFuncFunc               func(context.Context, database.TXFunc) error
}

func (f *fakeStore) DeployInstructionsLatestForFeature(context.Context, uuid.UUID, string) (*model.DeployInstruction, error) {
	panic("not called")
}

func (f *fakeStore) DeployInstructionUpdateStatus(context.Context, uuid.UUID, model.RolloutStatus) error {
	panic("not called")
}

func (f *fakeStore) EnvironmentCreate(context.Context, *model.EnvironmentCreate) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeStore) EnvironmentGet(context.Context, uuid.UUID) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeStore) EnvironmentIDByNames(ctx context.Context, t, e string) (uuid.UUID, error) {
	return f.environmentIDByNamesFunc(ctx, t, e)
}

func (f *fakeStore) ReleaseStatusCreateOrUpdate(context.Context, uuid.UUID, *message.Release) error {
	panic("not called")
}

func (f *fakeStore) TxFunc(ctx context.Context, fn database.TXFunc) error {
	return f.txFuncFunc(ctx, fn)
}
