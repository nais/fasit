package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/stretchr/testify/assert"
)

func Test_statusResolver_MissingDependencies(t *testing.T) {
	id := uuid.New()
	env := &model.Environment{ID: id, Kind: model.EnvironmentKindTenant}
	ctx := context.Background()

	featureName := &model.FeatureState{
		FeatureName: "featureName",
		Enabled:     true,
		EnvID:       env.ID,
	}
	deployedDep := &model.FeatureState{
		FeatureName: "deployedDep",
		Enabled:     true,
		EnvID:       env.ID,
	}
	pendingDep := &model.FeatureState{
		FeatureName: "pendingDep",
		Enabled:     true,
		EnvID:       env.ID,
	}
	notEnabledDep := &model.FeatureState{
		FeatureName: "notEnabledDep",
		Enabled:     false,
		EnvID:       env.ID,
	}
	enabledDep := &model.FeatureState{
		FeatureName: "enabledDep",
		Enabled:     true,
		EnvID:       env.ID,
	}

	features := []*model.FeatureState{featureName, deployedDep, pendingDep, notEnabledDep, enabledDep}

	repo := mocks.NewRepo(t)
	repo.On("FeatureStatesGet", ctx, id).Return(features, nil).Times(3)

	r := statusResolver{
		Resolver: &Resolver{
			Repo: repo,
		},
	}

	feats := []feature.Feature{
		{
			Name: featureName.FeatureName,
			FeatureYAML: feature.FeatureYAML{
				EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
				Dependencies: feature.Dependencies{feature.Dependency{
					AnyOf: []string{},
					AllOf: []string{deployedDep.FeatureName, pendingDep.FeatureName},
				}},
			},
		},
		{
			Name:             deployedDep.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
			DependsOn: feature.Dependencies{feature.Dependency{
				AnyOf: []string{enabledDep.FeatureName, notEnabledDep.FeatureName},
				AllOf: []string{},
			}},
		},
		{
			Name:             pendingDep.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		},
		{
			Name:             notEnabledDep.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		},
		{
			Name:             enabledDep.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		},
	}

	t.Run("Pending dep is missing", func(t *testing.T) {
		status := &model.Status{
			EnvironmentID: env.ID,
			Feature:       featureName.FeatureName,
		}
		got, err := r.MissingDependencies(ctx, status)

		assert.NoError(t, err)

		want := []*model.Feature{
			{
				Name:             pendingDep.FeatureName,
				EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
				Dependencies:     []*model.Dependency{},
				Config:           json.RawMessage("{}"),
			},
		}

		assert.Equal(t, want, got)
	})
	t.Run("one of AnyOf is running gives no missing dependencies", func(t *testing.T) {
		status := &model.Status{
			EnvironmentID: env.ID,
			Feature:       deployedDep.FeatureName,
		}

		got, err := r.MissingDependencies(ctx, status)
		assert.NoError(t, err)

		want := []*model.Feature{}

		assert.Equal(t, want, got)
	})
	t.Run("no dependencies gives no missing", func(t *testing.T) {
		status := &model.Status{
			EnvironmentID: env.ID,
			Feature:       enabledDep.FeatureName,
		}

		got, err := r.MissingDependencies(ctx, status)
		assert.NoError(t, err)

		want := []*model.Feature{}

		assert.Equal(t, want, got)
	})
}
