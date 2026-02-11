package rollout

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/environment/environmenttest"
	"github.com/nais/fasit/internal/feature/featuretest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatussql"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatustest"
	"github.com/nais/fasit/internal/rollout/rolloutsql"
	"github.com/nais/fasit/internal/rollout/rolloutsql/mocks"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"go.opentelemetry.io/otel/metric/noop"
)

var atTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)

type reconcileTestEnvironment struct {
	Environment     model.Environment
	TenantName      string
	NaisdReportedAt time.Time
	Status          []rolloutsql.DeployInstruction
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
				Status: []rolloutsql.DeployInstruction{
					{
						ID:             uuid.New(),
						FeatureName:    "feature1",
						FeatureVersion: "1",
						Hash:           "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
						Status:         model.RolloutStatusDeployed.String(),
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
				Status: []rolloutsql.DeployInstruction{
					{
						ID:             uuid.New(),
						FeatureName:    "feature1",
						FeatureVersion: "1",
						Hash:           "c5f057e78616cfea744cf031f52d7f772e00190d27383dbf6c0c6e7f128cf67b",
						Status:         model.RolloutStatusDeployed.String(),
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

			reconciler, err := NewReconciler(nil, nil, &mockNotifier{}, noop.NewMeterProvider().Meter("test"), logrus.NewEntry(logrus.StandardLogger()))
			if err != nil {
				t.Fatal(err)
			}

			te := []*model.TenantEnvironment{}
			for _, e := range tt.environments {
				te = append(te, &model.TenantEnvironment{
					TenantName:  e.TenantName,
					Environment: e.Environment,
				})
			}

			ctx = environmenttest.RegisterMock(ctx, t)
			ctx = featuretest.RegisterMock(ctx, t)

			environmenttest.OnTenantEnvironments(ctx, te)
			ctx = featuretest.OnHelmValues(ctx, uuid.Nil, "", nil)
			querier := mocks.NewQuerier(t)
			reconciler.querier = querier

			for _, te := range tt.environments {
				if len(tt.want) > 0 {
					querier.EXPECT().DeployInstructionsCreate(mock.Anything, mock.MatchedBy(func(params rolloutsql.DeployInstructionsCreateParams) bool {
						return params.EnvironmentID == te.Environment.ID
					})).Return(tt.want[0].ID, nil)
				}
				featuretest.OnFeaturesForKind(ctx, te.Environment.Kind, tt.features)

				querier.EXPECT().DeployInstructionsLatestForEnvironment(mock.Anything, te.Environment.ID).Return(te.Status, nil)
				featuretest.OnFeatureStatesGet(ctx, te.Environment.ID, te.FeatureStates)

				querier.EXPECT().RolloutAssignDeployInstruction(mock.Anything, mock.Anything).Return(nil).Maybe()

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
			publisher := func(topicID string, log logrus.FieldLogger) Publisher {
				return &mockPublisher{topicID: topicID, messages: &messages}
			}

			reconciler.publisher = publisher

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
