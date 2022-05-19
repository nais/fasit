package workers

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
)

var reconcileTests = map[string]struct {
	features []feature.Feature
	store    *mockStore
	want     []message.DeployInstruction
}{
	"all empty": {
		store: &mockStore{
			tenantEnvironments: []*model.TenantEnvironments{},
			status:             []*model.Status{},
		},
		want: []message.DeployInstruction{},
	},

	"no statuses": {
		features: []feature.Feature{
			{
				Name:    "feature1",
				Chart:   "somechart",
				Version: "1",
			},
		},
		store: &mockStore{
			featureStates: []*model.FeatureState{
				{
					FeatureName: "feature1",
					Enabled:     true,
				},
			},
			tenantEnvironments: []*model.TenantEnvironments{
				{
					Environment: model.Environment{
						Name: "prod",
					},
					TenantName: "tenant1",
				},
			},
			status: []*model.Status{},
		},
		want: []message.DeployInstruction{
			{
				Name:       "feature1",
				Version:    "1",
				Chart:      "somechart",
				ConfigHash: "6bceca2544c6cea880e6a21a64b348881714ed4dd7cc76218c8057fe77dfb6ca",
			},
		},
	},

	"1 feature without change": {
		features: []feature.Feature{
			{
				Name:    "feature1",
				Chart:   "somechart",
				Version: "1",
			},
		},
		store: &mockStore{
			featureStates: []*model.FeatureState{
				{
					FeatureName: "feature1",
					Enabled:     true,
				},
			},
			tenantEnvironments: []*model.TenantEnvironments{
				{
					Environment: model.Environment{
						Name: "prod",
					},
					TenantName: "tenant1",
				},
			},
			status: []*model.Status{
				{
					Feature:    "feature1",
					Version:    "1",
					ConfigHash: "6bceca2544c6cea880e6a21a64b348881714ed4dd7cc76218c8057fe77dfb6ca",
				},
			},
		},
		want: []message.DeployInstruction{},
	},

	"2 features 1 disabled": {
		features: []feature.Feature{
			{
				Name:    "feature1",
				Chart:   "somechart",
				Version: "1",
			},
			{
				Name:    "feature2",
				Chart:   "somechart",
				Version: "2",
			},
		},
		store: &mockStore{
			featureStates: []*model.FeatureState{
				{
					FeatureName: "feature1",
					Enabled:     true,
				},
				{
					FeatureName: "feature2",
					Enabled:     false,
				},
			},
			tenantEnvironments: []*model.TenantEnvironments{
				{
					Environment: model.Environment{
						Name: "prod",
					},
					TenantName: "tenant1",
				},
			},
			status: []*model.Status{},
		},
		want: []message.DeployInstruction{
			{
				Name:       "feature1",
				Version:    "1",
				Chart:      "somechart",
				ConfigHash: "6bceca2544c6cea880e6a21a64b348881714ed4dd7cc76218c8057fe77dfb6ca",
			},
		},
	},

	"2 features 1 change": {
		features: []feature.Feature{
			{
				Name:    "feature1",
				Chart:   "somechart",
				Version: "1",
			},
			{
				Name:    "feature2",
				Chart:   "somechart",
				Version: "2",
			},
		},
		store: &mockStore{
			featureStates: []*model.FeatureState{
				{
					FeatureName: "feature1",
					Enabled:     true,
				},
				{
					FeatureName: "feature2",
					Enabled:     true,
				},
			},
			tenantEnvironments: []*model.TenantEnvironments{
				{
					Environment: model.Environment{
						Name: "prod",
					},
					TenantName: "tenant1",
				},
			},
			status: []*model.Status{
				{
					Feature:    "feature1",
					Version:    "1",
					ConfigHash: "6bceca2544c6cea880e6a21a64b348881714ed4dd7cc76218c8057fe77dfb6ca",
				},
			},
		},
		want: []message.DeployInstruction{
			{
				Name:       "feature2",
				Version:    "2",
				Chart:      "somechart",
				ConfigHash: "20b62be5cb7ae8b60d6cf05e0de075cec01f56f9c7ff762b1b07fa8fe12ae8ff",
			},
		},
	},
}

func TestReconcile(t *testing.T) {
	for name, tt := range reconcileTests {
		t.Run(name, func(t *testing.T) {
			messages := []message.DeployInstruction{}
			recociler := &Reconciler{
				repo:       tt.store,
				featureMgr: &feature.Manager{Features: tt.features},
				publisher: func(projectID, topicID string, log *logrus.Entry) Publisher {
					return &mockPublisher{projectID: projectID, topicID: topicID, messages: &messages}
				},
				log:       logrus.NewEntry(logrus.StandardLogger()),
				projectID: "root-project",
			}

			ctx := context.Background()
			if err := recociler.reconcile(ctx); err != nil {
				t.Errorf("reconcile failed: %v", err)
			}

			if !cmp.Equal(tt.want, messages) {
				t.Errorf(cmp.Diff(tt.want, messages))
			}
		})
	}
}

type mockStore struct {
	tenantEnvironments []*model.TenantEnvironments
	status             []*model.Status
	featureStates      []*model.FeatureState
	helmValues         map[string]any
}

func (m *mockStore) TenantEnvironments(ctx context.Context) ([]*model.TenantEnvironments, error) {
	return m.tenantEnvironments, nil
}

func (m *mockStore) StatusForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]*model.Status, error) {
	return m.status, nil
}

func (m *mockStore) FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error) {
	return m.featureStates, nil
}

func (m *mockStore) HelmValues(ctx context.Context, feature *feature.Feature, envID uuid.UUID) (map[string]any, error) {
	return m.helmValues, nil
}

type mockPublisher struct {
	projectID string
	topicID   string
	messages  *[]message.DeployInstruction
}

func (m *mockPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	*m.messages = append(*m.messages, msg)
	return nil
}

func (m *mockPublisher) Stop() {}
