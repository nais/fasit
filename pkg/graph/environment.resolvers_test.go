package graph

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

func Test_environmentResolver_FeatureStates_FeatureStateMerge_Works(t *testing.T) {
	id := uuid.New()
	env := &model.Environment{ID: id, Kind: model.EnvironmentKindTenant}
	ctx := context.Background()

	repoFeatureStates := []*model.FeatureState{
		{
			FeatureName: "repo-feature",
			EnvID:       env.ID,
			Enabled:     true,
		},
	}

	repo := mocks.NewRepo(t)
	repo.On("FeatureStatesGet", ctx, id).Return(repoFeatureStates, nil).Once()

	r := environmentResolver{
		Resolver: &Resolver{
			Repo:     repo,
			Features: &feature.Manager{},
		},
	}

	r.Features.SetFeatures([]feature.Feature{
		{
			Name:             "global-feature",
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		},
		{
			Name:             "repo-feature",
			EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindTenant},
		},
	})

	got, err := r.FeatureStates(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.FeatureState{
		{
			FeatureName: "global-feature",
			EnvID:       env.ID,
			Enabled:     false,
		},
		{
			FeatureName: "repo-feature",
			EnvID:       env.ID,
			Enabled:     true,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}
