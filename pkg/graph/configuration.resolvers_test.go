package graph

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/mocks"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

func Test_queryResolver_Configuration_Early_Exit_When_EnvID_Set(t *testing.T) {
	feature := "feature"
	ctx := context.Background()
	envID := uuid.New()

	repo := mocks.NewRepo(t)
	repo.On("ConfigGetForEnv", ctx, feature, envID).Return(nil, nil).Once()

	r := &queryResolver{
		Resolver: &Resolver{
			Repo: repo,
		},
	}

	ret, err := r.Configuration(ctx, feature, &envID)
	if err != nil {
		t.Fatal(err)
	}

	if ret != nil {
		t.Fatal("expected nil (from mock repo)")
	}
}

func Test_queryResolver_Configuration_Empty_Defaults_Are_Set(t *testing.T) {
	featureName := "feature"
	ctx := context.Background()

	repo := mocks.NewRepo(t)
	mockConfig := []*model.Configuration{
		{
			Feature: "feature",
			Key:     "string",
			Value:   []byte("stringValue"),
		},
		{
			Feature: "feature",
			Key:     "int",
			Value:   []byte("intValue"),
		},
	}
	repo.On("ConfigGet", ctx, featureName).Return(mockConfig, nil).Once()

	r := &queryResolver{
		Resolver: &Resolver{
			Repo: repo,
			Features: &feature.Manager{
				Features: []feature.Feature{
					{
						Name: featureName,
						Config: feature.Config{
							"string": {
								Type: model.ConfigTypeString,
							},
							"bool": {
								Type: model.ConfigTypeBool,
							},
							"stringArray": {
								Type: model.ConfigTypeStringArray,
							},
							"int": {
								Type: model.ConfigTypeInt,
							},
						},
					},
				},
			},
		},
	}

	got, err := r.Configuration(ctx, featureName, nil)
	if err != nil {
		t.Fatalf("Configuration(ctx, %q, nil) err = %v, want nil", featureName, err)
	}

	want := []*model.Configuration{
		{Feature: "feature", Key: "string", Value: json.RawMessage("stringValue"), Type: model.ConfigTypeString},
		{Feature: "feature", Key: "int", Value: json.RawMessage("intValue"), Type: model.ConfigTypeInt},
		{Feature: "feature", Key: "bool", Value: json.RawMessage("null"), Type: model.ConfigTypeBool},
		{Feature: "feature", Key: "stringArray", Value: json.RawMessage("null"), Type: model.ConfigTypeStringArray},
	}

	if !cmp.Equal(want, got, cmpopts.SortSlices(func(a, b *model.Configuration) bool {
		return strings.Compare(a.Key, b.Key) < 0
	})) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}
