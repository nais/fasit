package workers

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/metric"
)

var atTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

type reconcileTestEnvironment struct {
	Environment     model.Environment
	TenantName      string
	NaisdReportedAt time.Time
	Status          []*model.Status
	FeatureStates   []*model.FeatureState
}

var reconcileTests = map[string]struct {
	features          []*model.Feature
	environmentHealth *model.Health
	environments      []*reconcileTestEnvironment
	want              []message.DeployInstruction
}{
	"all empty": {
		want: []message.DeployInstruction{},
	},

	"no statuses": {
		features: []*model.Feature{
			{
				Name:    "feature1",
				Chart:   "somechart",
				Version: "1",
			},
		},
		environments: []*reconcileTestEnvironment{
			{
				Environment: model.Environment{
					Name: "prod",
				},
				TenantName: "tenant1",
				FeatureStates: []*model.FeatureState{
					{
						FeatureName: "feature1",
						Enabled:     true,
						EnabledAt:   &atTime,
					},
				},
			},
		},
		want: []message.DeployInstruction{
			{
				Name:       "feature1",
				Version:    "1",
				Chart:      "somechart",
				ConfigHash: "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
			},
		},
	},

	"1 feature without change": {
		features: []*model.Feature{
			{
				Name:    "feature1",
				Chart:   "somechart",
				Version: "1",
			},
		},
		environments: []*reconcileTestEnvironment{
			{
				FeatureStates: []*model.FeatureState{
					{
						FeatureName: "feature1",
						Enabled:     true,
						EnabledAt:   &atTime,
					},
				},
				Environment: model.Environment{
					Name: "prod",
				},
				TenantName: "tenant1",
				Status: []*model.Status{
					{
						Feature:    "feature1",
						Version:    "1",
						ConfigHash: "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
					},
				},
			},
		},
		want: []message.DeployInstruction{},
	},

	"2 features 1 disabled": {
		features: []*model.Feature{
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
		environments: []*reconcileTestEnvironment{
			{
				Environment: model.Environment{
					Name: "prod",
				},
				TenantName: "tenant1",
				FeatureStates: []*model.FeatureState{
					{
						FeatureName: "feature1",
						Enabled:     true,
						EnabledAt:   &atTime,
					},
					{
						FeatureName: "feature2",
						Enabled:     false,
					},
				},
			},
		},
		want: []message.DeployInstruction{
			{
				Name:       "feature1",
				Version:    "1",
				Chart:      "somechart",
				ConfigHash: "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
			},
		},
	},

	"2 features 1 change": {
		features: []*model.Feature{
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
		environments: []*reconcileTestEnvironment{
			{
				Environment: model.Environment{
					Name: "prod",
				},
				TenantName: "tenant1",
				FeatureStates: []*model.FeatureState{
					{
						FeatureName: "feature1",
						Enabled:     true,
						EnabledAt:   &atTime,
					},
					{
						FeatureName: "feature2",
						Enabled:     true,
						EnabledAt:   &atTime,
					},
				},
				Status: []*model.Status{
					{
						Feature:    "feature1",
						Version:    "1",
						ConfigHash: "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
					},
				},
			},
		},
		want: []message.DeployInstruction{
			{
				Name:       "feature2",
				Version:    "2",
				Chart:      "somechart",
				ConfigHash: "c9138ce6414dda0e916cf4c8559fa2e1f285e4e101a49f4adc41ae2d3898388a",
			},
		},
	},
}

func TestReconcile(t *testing.T) {
	for name, tt := range reconcileTests {
		t.Run(name, func(t *testing.T) {
			repo := mocks.NewRepo(t)

			te := []*model.TenantEnvironment{}
			for _, e := range tt.environments {
				te = append(te, &model.TenantEnvironment{
					TenantName:  e.TenantName,
					Environment: e.Environment,
				})
			}
			repo.On("TenantEnvironments", mock.Anything).Return(te, nil)

			for _, te := range tt.environments {
				repo.On("FeaturesForKind", mock.Anything, te.Environment.Kind, te.Environment.CI).Return(tt.features, nil)
				repo.On("StatusForEnvironment", mock.Anything, te.Environment.ID).Return(te.Status, nil)
				repo.On("FeatureStatesGet", mock.Anything, te.Environment.ID).Return(te.FeatureStates, nil)
				repo.On("HelmValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil).Maybe()

				reportAt := time.Now()
				if !te.NaisdReportedAt.IsZero() {
					reportAt = te.NaisdReportedAt
				}
				repo.On("HealthGet", mock.Anything, te.Environment.ID).Return(&model.Health{
					ReportedAt: reportAt,
				}, nil)
			}

			messages := []message.DeployInstruction{}

			publisher := func(projectID, topicID string, log *logrus.Entry) Publisher {
				return &mockPublisher{projectID: projectID, topicID: topicID, messages: &messages}
			}

			reconciler, err := NewReconciler(repo, publisher, "root-project", metric.NewNoopMeter(), logrus.NewEntry(logrus.StandardLogger()))
			if err != nil {
				t.Fatal(err)
			}

			ctx := context.Background()
			if err := reconciler.reconcile(ctx); err != nil {
				t.Errorf("reconcile failed: %v", err)
			}

			if !cmp.Equal(tt.want, messages) {
				t.Errorf(cmp.Diff(tt.want, messages))
			}
		})
	}
}

func TestReconcile_AutoInstall(t *testing.T) {
	// tests := map[string]struct {
	// 	features        []feature.Feature
	// 	expectedFeature string
	// 	status          map[string]*model.Status
	// 	states          map[string]*model.FeatureState
	// }{
	// 	"no features": {
	// 		features:        []feature.Feature{},
	// 		expectedFeature: "",
	// 	},
	// 	"one feature": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "feature1",
	// 	},
	// 	"one feature which is deployed": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "",
	// 		status: map[string]*model.Status{
	// 			"feature1": {
	// 				Feature: "feature1",
	// 				Status:  model.RolloutStatusDeployed,
	// 			},
	// 		},
	// 	},
	// 	"one feature which is already enabled": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "",
	// 		states: map[string]*model.FeatureState{
	// 			"feature1": {
	// 				FeatureName: "feature1",
	// 				Enabled:     true,
	// 			},
	// 		},
	// 	},
	// 	"one feature which is failed": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "",
	// 		status: map[string]*model.Status{
	// 			"feature1": {
	// 				Feature: "feature1",
	// 				Status:  model.RolloutStatusFailed,
	// 			},
	// 		},
	// 	},
	// 	"one feature which is pending": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "",
	// 		status: map[string]*model.Status{
	// 			"feature1": {
	// 				Feature: "feature1",
	// 				Status:  model.RolloutStatusPending,
	// 			},
	// 		},
	// 	},
	// 	"one tenant feature, one management feature": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature_tenant",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 			{
	// 				Name:        "feature_mgmt",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "feature_tenant",
	// 	},
	// 	"successfull complex case with dependencies and statuses": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 				DependsOn:   feature.Dependencies{{AllOf: []string{"feature2"}}},
	// 			},
	// 			{
	// 				Name:        "feature2",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 				DependsOn:   feature.Dependencies{{AllOf: []string{"feature3"}}},
	// 			},
	// 			{
	// 				Name:        "feature3",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		expectedFeature: "feature2",
	// 		status: map[string]*model.Status{
	// 			"feature1": {
	// 				Feature: "feature1",
	// 				Status:  model.RolloutStatusDeployed,
	// 			},
	// 			"feature3": {
	// 				Feature: "feature3",
	// 				Status:  model.RolloutStatusDeployed,
	// 			},
	// 		},
	// 	},
	// 	"pending complex case with dependencies and statuses": {
	// 		features: []feature.Feature{
	// 			{
	// 				Name:        "feature1",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 				DependsOn:   feature.Dependencies{{AllOf: []string{"feature2"}}},
	// 			},
	// 			{
	// 				Name:        "feature2",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 				DependsOn:   feature.Dependencies{{AllOf: []string{"feature3"}}},
	// 			},
	// 			{
	// 				Name:        "feature3",
	// 				AutoInstall: []model.EnvironmentKind{model.EnvironmentKindTenant},
	// 			},
	// 		},
	// 		status: map[string]*model.Status{
	// 			"feature3": {
	// 				Feature: "feature3",
	// 				Status:  model.RolloutStatusPending,
	// 			},
	// 		},
	// 	},
	// }

	// te := &model.TenantEnvironments{
	// 	Environment: model.Environment{
	// 		ID:   uuid.New(),
	// 		Kind: model.EnvironmentKindTenant,
	// 	},
	// }

	// for name, tt := range tests {
	// 	t.Run(name, func(t *testing.T) {
	// 		repo := mocks.NewRepo(t)
	// 		if tt.expectedFeature != "" {
	// 			repo.On("FeatureStatesCreateOrUpdate",
	// 				mock.Anything, te.ID, mock.MatchedBy(func(f *feature.Feature) bool { return f.Name == tt.expectedFeature }), true).Return(nil, nil)
	// 		}

	// 		recociler := &Reconciler{
	// 			repo: repo,
	// 			log:  logrus.NewEntry(logrus.StandardLogger()),
	// 		}

	// 		ctx := context.Background()
	// 		if err := recociler.autoInstallNextFeature(ctx, te, tt.features, tt.status, tt.states); err != nil {
	// 			t.Errorf("reconcile failed: %v", err)
	// 		}
	// 	})
	// }
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
