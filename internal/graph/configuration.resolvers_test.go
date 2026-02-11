package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/mocks"
	"github.com/nais/fasit/internal/environment/environmenttest"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featuretest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func Test_queryResolver_Configuration_With_Environment_ID(t *testing.T) {
	ctx := context.Background()
	envID := uuid.New()

	r := &queryResolver{}

	got, err := r.Configuration(ctx, "feature", &envID)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Configurations{
		EnvID:       &envID,
		FeatureName: "feature",
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func Test_queryResolver_Configuration_Global_Configurations(t *testing.T) {
	ctx := context.Background()

	feature := &model.Feature{
		Name: "feature",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"key.set": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
				"another.key": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
			},
		},
		ValuesYAML: map[string]json.RawMessage{
			"another.key": json.RawMessage(`"helm_value"`),
		},
	}

	storedConfigID := uuid.New()

	repo := mocks.NewRepo(t)
	retConfig := []*model.Configuration{
		{
			ID:        storedConfigID,
			Key:       "key.set",
			Source:    model.ConfigSourceGlobal,
			Content:   json.RawMessage(`"value"`),
			GraphVars: model.ConfigurationGraphVars{FeatureName: feature.Name},
		},
	}
	ctx = featuretest.RegisterMock(ctx, t)
	featuretest.OnConfigGet(ctx, feature.Name, retConfig)
	featuretest.OnFeatureByName(ctx, feature.Name, feature)

	r := &queryResolver{
		Resolver: &Resolver{
			Repo: repo,
		},
	}

	got, err := r.Configurations().Configuration(ctx, &model.Configurations{FeatureName: feature.Name})
	if err != nil {
		t.Fatalf("Configuration(ctx, %q, nil) err = %v, want nil", feature.Name, err)
	}

	want := []*model.Configuration{
		{
			ID:        storedConfigID,
			Value:     &model.Value{Config: &model.Config{Type: model.ConfigTypeString}, GraphQLKey: "key.set"},
			Content:   json.RawMessage(`"value"`),
			Source:    model.ConfigSourceGlobal,
			Key:       "key.set",
			GraphVars: model.ConfigurationGraphVars{FeatureName: feature.Name},
		},
		{
			ID:        fakeUUID(feature.Name, "another.key"),
			Value:     &model.Value{Config: &model.Config{Type: model.ConfigTypeString}, GraphQLKey: "another.key"},
			Content:   json.RawMessage(`"helm_value"`),
			Source:    model.ConfigSourceHelm,
			Key:       "another.key",
			GraphVars: model.ConfigurationGraphVars{FeatureName: feature.Name},
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func Test_queryResolver_Configuration_Feature_Configurations(t *testing.T) {
	ctx := context.Background()

	env := &model.Environment{
		ID:   uuid.New(),
		Name: "env",
	}

	feature := &model.Feature{
		Name: "feature",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"key.set": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
				"key.env": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
				"another.key": model.Value{
					Config: &model.Config{
						Type: model.ConfigTypeString,
					},
				},
			},
		},
		ValuesYAML: map[string]json.RawMessage{
			"another.key": json.RawMessage(`"helm_value"`),
		},
	}

	repo := mocks.NewRepo(t)
	repo.On("EnvConfig", mock.Anything, mock.IsType(feature), env.ID).Return([]*model.Configuration{
		{
			ID:      uuid.New(),
			Key:     "key.set",
			Source:  model.ConfigSourceGlobal,
			Content: json.RawMessage(`"value"`),
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
		{
			ID:      uuid.New(),
			Key:     "key.env",
			Source:  model.ConfigSourceEnv,
			Content: json.RawMessage(`"env"`),
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
		{
			ID:      uuid.New(),
			Key:     "key.unknown",
			Source:  model.ConfigSourceEnv,
			Content: json.RawMessage(`"shouldn't be set"`),
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
	}, nil).Once()

	ctx = featuretest.RegisterMock(ctx, t)
	ctx = environmenttest.RegisterMock(ctx, t)

	featuretest.OnFeatureByNameForEnv(ctx, env.Name, feature)
	repo.On("EnvironmentGet", mock.Anything, env.ID).Return(env, nil).Once()

	r := &queryResolver{
		Resolver: &Resolver{
			Repo: repo,
		},
	}

	c := &model.Configurations{FeatureName: feature.Name, EnvID: &env.ID}
	got, err := r.Configurations().Configuration(ctx, c)
	if err != nil {
		t.Fatalf("Configuration(ctx, %q, envID) err = %v, want nil", feature.Name, err)
	}

	want := []*model.Configuration{
		{
			Value:   &model.Value{GraphQLKey: "key.unknown"},
			Content: json.RawMessage(`"shouldn't be set"`),
			Source:  model.ConfigSourceUnknown,
			Key:     "key.unknown",
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
		{
			Value: &model.Value{
				Config:     &model.Config{Type: model.ConfigTypeString},
				GraphQLKey: "key.env",
			},
			Content: json.RawMessage(`"env"`),
			Source:  model.ConfigSourceEnv,
			Key:     "key.env",
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
		{
			Value: &model.Value{
				Config:     &model.Config{Type: model.ConfigTypeString},
				GraphQLKey: "key.set",
			},
			Content: json.RawMessage(`"value"`),
			Source:  model.ConfigSourceGlobal,
			Key:     "key.set",
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
		{
			Value: &model.Value{
				Config:     &model.Config{Type: model.ConfigTypeString},
				GraphQLKey: "another.key",
			},
			Content: json.RawMessage(`"helm_value"`),
			Source:  model.ConfigSourceHelm,
			Key:     "another.key",
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   feature.Name,
				EnvironmentID: &env.ID,
			},
		},
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func Test_queryResolver_Configuration_Feature_Computed(t *testing.T) {
	ctx := context.Background()
	env := &model.Environment{
		ID:   uuid.New(),
		Kind: model.EnvironmentKindTenant,
	}

	f := &model.Feature{
		Name: "feature",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"key.computed.one": model.Value{
					Computed: &model.Computed{
						Template: `{{ .Env.value }}`,
					},
				},
			},
		},
	}

	repo := mocks.NewRepo(t)

	ctx = featuretest.RegisterMock(ctx, t)
	ctx = environmenttest.RegisterMock(ctx, t)

	featuretest.OnFeatureByNameForEnv(ctx, env.Name, f)

	repo.On("MappingValuesForEnvironment", mock.Anything, env.ID, false).Return(
		&feature.ComputedValues{
			Kind: env.Kind,
			Env: map[string]any{
				"value": "computed value",
			},
		},
		env.Kind,
		nil,
	)

	r := &queryResolver{
		Resolver: &Resolver{
			Repo: repo,
		},
	}

	c := &model.Configurations{FeatureName: f.Name, EnvID: &env.ID}
	got, err := r.Configurations().Computed(ctx, c)
	if err != nil {
		t.Fatalf("Computed(ctx, %q, envID) err = %v, want nil", f.Name, err)
	}

	want := []*model.ComputedValue{
		{
			Value: &model.Value{
				Computed: &model.Computed{
					Template: "{{ .Env.value }}",
				},
				GraphQLKey: "key.computed.one",
			},
			Content: json.RawMessage(`"computed value"`),
		},
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}
