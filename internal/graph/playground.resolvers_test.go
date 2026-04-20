package graph

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/mocks"
	"github.com/nais/fasit/internal/environment/environmenttest"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featuretest"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
	"gopkg.in/yaml.v3"
)

func Test_Playground_FeatureName_UsedForDBLookup(t *testing.T) {
	ctx := context.Background()
	envID := uuid.New()

	helmVals := map[string]any{
		"memory": json.RawMessage(`"256Mi"`),
	}
	// Stub HelmValuesFuncKey to verify that featureName is passed to the resolver
	helmValuesFunc := func(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
		if f.Name != "console-frontend" {
			t.Errorf("expected featureName=console-frontend, got %s", f.Name)
		}
		return helmVals, nil
	}
	ctx = context.WithValue(ctx, feature.HelmValuesFuncKey, helmValuesFunc)

	repo := mocks.NewRepo(t)
	repo.EXPECT().EnvironmentByNames(mock.Anything, "nav", "management").Return(&model.Environment{
		ID:   envID,
		Kind: model.EnvironmentKindManagement,
	}, nil).Once()

	r := &mutationResolver{Resolver: &Resolver{Repo: repo}}

	result, err := r.Playground(ctx, model.PlaygroundInput{
		TenantSlug:  "nav",
		EnvSlug:     "management",
		FeatureName: new("console-frontend"),
		Code:        "environmentKinds:\n  - management\nvalues:\n  memory:\n    config:\n      type: string\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected playground errors: %v", result.Errors)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(*result.Result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["memory"] != "256Mi" {
		t.Errorf("expected memory=256Mi, got %v", out["memory"])
	}
}

func Test_Playground_IncludeUnsetConfig_DoesNotOverwriteExistingValues(t *testing.T) {
	ctx := context.Background()
	envID := uuid.New()

	helmVals := map[string]any{
		"memory": json.RawMessage(`"256Mi"`),
	}
	ctx = featuretest.OnHelmValues(ctx, envID, "", helmVals)

	repo := mocks.NewRepo(t)
	repo.EXPECT().EnvironmentByNames(mock.Anything, "nav", "management").Return(&model.Environment{
		ID:   envID,
		Kind: model.EnvironmentKindManagement,
	}, nil).Once()

	r := &mutationResolver{Resolver: &Resolver{Repo: repo}}

	result, err := r.Playground(ctx, model.PlaygroundInput{
		TenantSlug:         "nav",
		EnvSlug:            "management",
		IncludeUnsetConfig: new(true),
		Code:               "environmentKinds:\n  - management\nvalues:\n  memory:\n    config:\n      type: string\n  cpu:\n    config:\n      type: string\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected playground errors: %v", result.Errors)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(*result.Result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["memory"] != "256Mi" {
		t.Errorf("includeUnsetConfig overwrote existing value: expected memory=256Mi, got %v", out["memory"])
	}
	if _, ok := out["cpu"]; !ok {
		t.Errorf("expected unset key 'cpu' to be present as null")
	}
	if out["cpu"] != nil {
		t.Errorf("expected unset key 'cpu' to be null, got %v", out["cpu"])
	}
}

func Test_Playground_JSONRawMessage_EncodesAsString(t *testing.T) {
	ctx := context.Background()
	envID := uuid.New()

	helmVals := map[string]any{
		"token": json.RawMessage(`"secret-value"`),
	}
	ctx = featuretest.OnHelmValues(ctx, envID, "", helmVals)

	repo := mocks.NewRepo(t)
	repo.EXPECT().EnvironmentByNames(mock.Anything, "nav", "management").Return(&model.Environment{
		ID:   envID,
		Kind: model.EnvironmentKindManagement,
	}, nil).Once()

	r := &mutationResolver{Resolver: &Resolver{Repo: repo}}

	result, err := r.Playground(ctx, model.PlaygroundInput{
		TenantSlug: "nav",
		EnvSlug:    "management",
		Code:       "environmentKinds:\n  - management\nvalues:\n  token:\n    config:\n      type: string\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected playground errors: %v", result.Errors)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(*result.Result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["token"] != "secret-value" {
		t.Errorf("expected token=secret-value (string), got %T %v", out["token"], out["token"])
	}
}

func Test_Playground_StripNoValue_RendersMissingComputedFieldsAsNull(t *testing.T) {
	ctx := context.Background()
	envID := uuid.New()

	// Simulate HelmValues returning <no value> (unquoted missing key) and nil (quoted missing key).
	helmVals := map[string]any{
		"present": json.RawMessage(`"hello"`),
		"missing": "<no value>",
		"quoted":  nil,
		"parent":  map[string]any{"child": "<no value>"},
	}
	ctx = featuretest.OnHelmValues(ctx, envID, "", helmVals)

	repo := mocks.NewRepo(t)
	repo.EXPECT().EnvironmentByNames(mock.Anything, "nav", "management").Return(&model.Environment{
		ID:   envID,
		Kind: model.EnvironmentKindManagement,
	}, nil).Once()

	r := &mutationResolver{Resolver: &Resolver{Repo: repo}}

	result, err := r.Playground(ctx, model.PlaygroundInput{
		TenantSlug: "nav",
		EnvSlug:    "management",
		Code:       "environmentKinds:\n  - management\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected playground errors: %v", result.Errors)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(*result.Result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out["present"] != "hello" {
		t.Errorf("expected present=hello, got %v", out["present"])
	}
	if _, ok := out["missing"]; !ok {
		t.Fatal("expected 'missing' key to be present")
	}
	if out["missing"] != nil {
		t.Errorf("expected 'missing' key to render as null, got %v", out["missing"])
	}
	if _, ok := out["quoted"]; !ok {
		t.Fatal("expected 'quoted' key to be present")
	}
	if out["quoted"] != nil {
		t.Errorf("expected 'quoted' key to render as null, got %v", out["quoted"])
	}
	parent, ok := out["parent"].(map[string]any)
	if !ok {
		t.Fatalf("expected 'parent' to be a map, got %T", out["parent"])
	}
	if _, ok := parent["child"]; !ok {
		t.Fatal("expected 'parent.child' key to be present")
	}
	if parent["child"] != nil {
		t.Errorf("expected 'parent.child' to render as null, got %v", parent["child"])
	}
}

func Test_Playground_IncludeChartDefaults_MergesDefaults(t *testing.T) {
	ctx := context.Background()
	ctx = featuretest.RegisterMock(ctx, t)
	ctx = environmenttest.RegisterMock(ctx, t)
	envID := uuid.New()

	helmVals := map[string]any{
		"resources": map[string]any{
			"requests": map[string]any{
				"memory": nil,
			},
		},
		"token": "overridden",
	}
	ctx = featuretest.OnHelmValues(ctx, envID, "", helmVals)

	featuretest.OnFeatureByNameForEnv(ctx, "management", &model.Feature{
		Name: "console-frontend",
		ValuesYAML: map[string]json.RawMessage{
			"resources.requests.memory": json.RawMessage(`"128Mi"`),
			"token":                     json.RawMessage(`"default-token"`),
			"replicaCount":              json.RawMessage(`2`),
		},
	})

	repo := mocks.NewRepo(t)
	repo.EXPECT().EnvironmentByNames(mock.Anything, "nav", "management").Return(&model.Environment{
		ID:   envID,
		Kind: model.EnvironmentKindManagement,
		Name: "management",
	}, nil).Once()

	r := &mutationResolver{Resolver: &Resolver{Repo: repo}}

	result, err := r.Playground(ctx, model.PlaygroundInput{
		TenantSlug:           "nav",
		EnvSlug:              "management",
		FeatureName:          new("console-frontend"),
		IncludeChartDefaults: new(true),
		IncludeUnsetConfig:   new(false),
		Code:                 "environmentKinds:\n  - management\n",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected playground errors: %v", result.Errors)
	}

	var out map[string]any
	if err := yaml.Unmarshal([]byte(*result.Result), &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	resources, ok := out["resources"].(map[string]any)
	if !ok {
		t.Fatalf("expected resources to be a map, got %T", out["resources"])
	}
	requests, ok := resources["requests"].(map[string]any)
	if !ok {
		t.Fatalf("expected resources.requests to be a map, got %T", resources["requests"])
	}
	if requests["memory"] != "128Mi" {
		t.Errorf("expected resources.requests.memory from defaults, got %v", requests["memory"])
	}

	if out["token"] != "overridden" {
		t.Errorf("expected resolved token to override default, got %v", out["token"])
	}

	if out["replicaCount"] != 2 {
		t.Errorf("expected replicaCount from defaults, got %v", out["replicaCount"])
	}
}
