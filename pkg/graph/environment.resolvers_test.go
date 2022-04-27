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
	env := &model.Environment{ID: id, Kind: model.EnvironmentKindPartner}
	ctx := context.Background()

	repoFeatureStates := []*model.FeatureState{
		{
			FeatureName: "repo-feature",
			Enabled:     true,
		},
	}

	repo := mocks.NewRepo(t)
	repo.On("FeatureStatesGet", ctx, id).Return(repoFeatureStates, nil).Once()

	r := environmentResolver{
		Resolver: &Resolver{
			Repo: repo,
			Features: &feature.Manager{
				Features: []feature.Feature{
					{
						Name:             "global-feature",
						EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindPartner},
					},
					{
						Name:             "repo-feature",
						EnvironmentKinds: []model.EnvironmentKind{model.EnvironmentKindPartner},
					},
				},
			},
		},
	}

	got, err := r.FeatureStates(ctx, env)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.FeatureState{
		{
			FeatureName: "repo-feature",
			Enabled:     true,
		},
		{
			FeatureName: "global-feature",
			Enabled:     false,
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}
