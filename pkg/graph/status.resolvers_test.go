package graph

import (
	"context"
	"encoding/json"
	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"testing"
)

func Test_statusResolver_MissingDependencies(t *testing.T) {
	id := uuid.New()
	env := &model.Environment{ID: id, Kind: model.EnvironmentKindTenant}
	ctx := context.Background()

	featureName := &model.FeatureState{
		FeatureName:   "featureName",
		Enabled:       true,
		RolloutStatus: model.RolloutStatusDeployed,
		EnvID:         env.ID,
	}
	deployedDep := &model.FeatureState{
		FeatureName:   "deployedDep",
		Enabled:       true,
		RolloutStatus: model.RolloutStatusDeployed,
		EnvID:         env.ID,
	}
	pendingDep := &model.FeatureState{
		FeatureName:   "pendingDep",
		Enabled:       true,
		RolloutStatus: model.RolloutStatusPending,
		EnvID:         env.ID,
	}
	notEnabledDep := &model.FeatureState{
		FeatureName:   "notEnabledDep",
		Enabled:       false,
		RolloutStatus: model.RolloutStatusDeployed,
		EnvID:         env.ID,
	}
	enabledDep := &model.FeatureState{
		FeatureName:   "enabledDep",
		Enabled:       true,
		RolloutStatus: model.RolloutStatusDeployed,
		EnvID:         env.ID,
	}

	features := []*model.FeatureState{featureName, deployedDep, pendingDep, notEnabledDep, enabledDep}

	repo := mocks.NewRepo(t)
	repo.On("FeatureStatesGet", ctx, id).Return(features, nil).Twice()

	r := statusResolver{
		Resolver: &Resolver{
			Repo:     repo,
			Features: &feature.Manager{},
		},
	}

	r.Features.SetFeatures([]feature.Feature{
		{
			Name:             featureName.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
			DependsOn: feature.Dependencies{feature.Dependency{
				AnyOf: []string{},
				AllOf: []string{deployedDep.FeatureName, pendingDep.FeatureName},
			}},
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
		}, {
			Name:             notEnabledDep.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		},
	})

	status := &model.Status{
		EnvironmentID: env.ID,
		Feature:       featureName.FeatureName,
	}
	got, err := r.MissingDependencies(ctx, status)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.Feature{
		{
			Name:             pendingDep.FeatureName,
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
			DependsOn:        []*model.Dependency{},
			Config:           json.RawMessage("{}"),
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}

	status2 := &model.Status{
		EnvironmentID: env.ID,
		Feature:       deployedDep.FeatureName,
	}

	got2, err := r.MissingDependencies(ctx, status2)
	if err != nil {
		t.Fatal(err)
	}

	want2 := []*model.Feature{}

	if !cmp.Equal(want2, got2) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want2, got2))
	}
}
