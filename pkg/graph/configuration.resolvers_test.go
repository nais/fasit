package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	feature "github.com/nais/fasit/pkg/feature2"
	"github.com/nais/fasit/pkg/graph/model"
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
	repo.On("ConfigGet", mock.Anything, feature.Name).Return([]*model.Configuration{
		{
			ID:        storedConfigID,
			Key:       "key.set",
			Source:    model.ConfigSourceGlobal,
			Content:   json.RawMessage(`"value"`),
			GraphVars: model.ConfigurationGraphVars{FeatureName: feature.Name},
		},
	}, nil).Once()
	repo.On("FeatureByName", mock.Anything, feature.Name).Return(feature, nil).Once()

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
	repo.On("EnvConfig", mock.Anything, feature.Name, env.ID).Return([]*model.Configuration{
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
	repo.On("FeatureByNameForEnv", mock.Anything, feature.Name, env.ID).Return(feature, nil).Once()

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
		Name: "env",
		Kind: model.EnvironmentKindTenant,
	}

	f := &model.Feature{
		Name: "feature",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"key.computed.one": model.Value{
					Computed: &model.Computed{
						Template: `{{ .Values.value }}`,
					},
				},
			},
		},
	}

	repo := mocks.NewRepo(t)
	repo.On("EnvironmentGet", mock.Anything, env.ID).Return(env, nil).Once()
	repo.On("EnvConfig", mock.Anything, f.Name, env.ID).Return([]*model.Configuration{}, nil).Once()
	repo.On("FeatureByNameForEnv", mock.Anything, f.Name, env.ID).Return(f, nil).Once()
	repo.On("MappingValuesForEnvironment", mock.Anything, env.ID, false).Return(
		&feature.ComputedValues{
			Kind: env.Kind,
			Env: map[string]any{
				"value": "computed value",
			},
		},
		env.Kind,
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

	want := []*model.Configuration{
		{
			Value:   &model.Value{GraphQLKey: "key.unknown"},
			Content: json.RawMessage(`"shouldn't be set"`),
			Source:  model.ConfigSourceUnknown,
			Key:     "key.unknown",
			GraphVars: model.ConfigurationGraphVars{
				FeatureName:   f.Name,
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
				FeatureName:   f.Name,
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
				FeatureName:   f.Name,
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
				FeatureName:   f.Name,
				EnvironmentID: &env.ID,
			},
		},
	}

	opts := cmpopts.IgnoreFields(model.Configuration{}, "ID")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}
