package workers_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/database/mocks"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/feature/featuretest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatussql"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatustest"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/metric/noop"
)

var atTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

type reconcileTestEnvironment struct {
	Environment     model.Environment
	TenantName      string
	NaisdReportedAt time.Time
	Status          []*model.DeployInstruction
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
				ID:         uuid.New(),
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
				Status: []*model.DeployInstruction{
					{
						ID:             uuid.New(),
						FeatureName:    "feature1",
						FeatureVersion: "1",
						Hash:           "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
						Status:         model.RolloutStatusDeployed,
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
				ID:         uuid.New(),
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
				Status: []*model.DeployInstruction{
					{
						ID:             uuid.New(),
						FeatureName:    "feature1",
						FeatureVersion: "1",
						Hash:           "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
						Status:         model.RolloutStatusDeployed,
					},
				},
			},
		},
		want: []message.DeployInstruction{
			{
				ID:         uuid.New(),
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
			ctx := context.Background()
			ctx = naisdstatustest.RegisterMock(ctx, t)
			statusQuerier := naisdstatustest.GetQuerier(ctx)

			repo := mocks.NewRepo(t)

			te := []*model.TenantEnvironment{}
			for _, e := range tt.environments {
				te = append(te, &model.TenantEnvironment{
					TenantName:  e.TenantName,
					Environment: e.Environment,
				})
			}

			repo.On("TenantEnvironments", mock.Anything, true).Return(te, nil)

			ctx = featuretest.RegisterMock(ctx, t)

			for _, te := range tt.environments {
				if len(tt.want) > 0 {
					repo.EXPECT().DeployInstructionCreate(mock.Anything, te.Environment.ID, mock.IsType(&model.Feature{}), mock.IsType(""), mock.IsType(&uuid.UUID{})).Return(tt.want[0].ID, nil).Once()
				}
				// repo.On("FeaturesForKind", mock.Anything, te.Environment.Kind, te.Environment.CI).Return(tt.features, nil)
				featuretest.OnFeaturesForKind(ctx, te.Environment.Kind, tt.features)

				repo.On("DeployInstructionsLatestForEnvironment", mock.Anything, te.Environment.ID).Return(te.Status, nil)
				repo.On("FeatureStatesGet", mock.Anything, te.Environment.ID).Return(te.FeatureStates, nil)
				repo.On("HelmValues", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil).Maybe()
				repo.EXPECT().RolloutAssignDeployInstruction(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

				reportAt := time.Now()
				if !te.NaisdReportedAt.IsZero() {
					reportAt = te.NaisdReportedAt
				}
				statusQuerier.On("Get", mock.Anything, te.Environment.ID).Return(naisdstatussql.HealthStatus{
					EnvironmentID: te.Environment.ID,
					ReportedAt: pgtype.Timestamptz{
						Time:  reportAt,
						Valid: true,
					},
				}, nil)
			}

			messages := []message.DeployInstruction{}

			meter := noop.NewMeterProvider().Meter("test")
			publisher := func(topicID string, log logrus.FieldLogger) workers.Publisher {
				return &mockPublisher{topicID: topicID, messages: &messages}
			}

			reconciler, err := workers.NewReconciler(repo, publisher, &mockNotifier{}, meter, logrus.NewEntry(logrus.StandardLogger()))
			if err != nil {
				t.Fatal(err)
			}

			if err := reconciler.Reconcile(ctx); err != nil {
				t.Errorf("reconcile failed: %v", err)
			}

			if !cmp.Equal(tt.want, messages) {
				t.Error(cmp.Diff(tt.want, messages))
			}
		})
	}
}

type mockPublisher struct {
	topicID  string
	messages *[]message.DeployInstruction
}

func (m *mockPublisher) Publish(ctx context.Context, msg message.DeployInstruction) error {
	*m.messages = append(*m.messages, msg)
	return nil
}

func (m *mockPublisher) Stop() {}

type mockNotifier struct{}

func (m *mockNotifier) Listen(table string, filters ...notifier.Filter) <-chan notifier.Payload {
	return nil
}
