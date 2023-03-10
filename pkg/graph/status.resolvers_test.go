package graph

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func Test_statusResolver_MissingDependencies(t *testing.T) {
	id := uuid.New()
	env := &model.Environment{ID: id, Kind: model.EnvironmentKindTenant}
	ctx := context.Background()

	anyof := &model.FeatureState{
		FeatureName: "anyof",
		Enabled:     true,
		EnvID:       env.ID,
	}
	allof := &model.FeatureState{
		FeatureName: "allof",
		Enabled:     true,
		EnvID:       env.ID,
	}
	enabledDep := &model.FeatureState{
		FeatureName: "enabledDep",
		Enabled:     true,
		EnvID:       env.ID,
	}
	notEnabledDep := &model.FeatureState{
		FeatureName: "notEnabledDep",
		Enabled:     false,
		EnvID:       env.ID,
	}

	features := []*model.FeatureState{anyof, notEnabledDep, enabledDep}

	repo := mocks.NewRepo(t)
	repo.On("FeatureStatesGet", ctx, id).Return(features, nil)

	feats := map[string]*model.Feature{
		anyof.FeatureName: {
			Name: anyof.FeatureName,
			FeatureYAML: model.FeatureYAML{
				EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
				Dependencies: model.Dependencies{model.Dependency{
					AnyOf: []string{enabledDep.FeatureName, notEnabledDep.FeatureName},
					AllOf: []string{},
				}},
			},
		},
		allof.FeatureName: {
			Name: allof.FeatureName,
			FeatureYAML: model.FeatureYAML{
				EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
				Dependencies: model.Dependencies{model.Dependency{
					AnyOf: []string{},
					AllOf: []string{enabledDep.FeatureName, notEnabledDep.FeatureName},
				}},
			},
		},
		notEnabledDep.FeatureName: {
			Name: notEnabledDep.FeatureName,
			FeatureYAML: model.FeatureYAML{
				EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
			},
		},
		enabledDep.FeatureName: {
			Name: enabledDep.FeatureName,
			FeatureYAML: model.FeatureYAML{
				EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
			},
		},
	}

	repo.On("FeatureByName", ctx, mock.AnythingOfType("string")).Return(func(ctx context.Context, name string) (*model.Feature, error) {
		return feats[name], nil
	}, nil)

	r := statusResolver{
		Resolver: &Resolver{
			Repo: repo,
		},
	}
	t.Run("having one of AnyOf gives no missing dependencies", func(t *testing.T) {
		status := &model.Status{
			EnvironmentID: env.ID,
			Feature:       anyof.FeatureName,
		}

		got, err := r.MissingDependencies(ctx, status)
		assert.NoError(t, err)

		want := []*model.Feature{}

		assert.Equal(t, want, got)
	})
	t.Run("having one of AllOf gives one missing dependency", func(t *testing.T) {
		status := &model.Status{
			EnvironmentID: env.ID,
			Feature:       allof.FeatureName,
		}

		got, err := r.MissingDependencies(ctx, status)
		assert.NoError(t, err)

		want := []*model.Feature{feats[notEnabledDep.FeatureName]}

		assert.Equal(t, want, got)
	})
}
