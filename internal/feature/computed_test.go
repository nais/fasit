package feature

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/internal/graph/model"
)

func TestGenerate_MissingMappingValues(t *testing.T) {
	tests := map[string]struct {
		template string
		want     map[string]any
	}{
		"missing key without quote is omitted": {
			template: "{{ .Env.missing_key }}",
			want:     map[string]any{},
		},
		"missing key with quote is omitted": {
			template: `{{ .Env.missing_key | quote }}`,
			want:     map[string]any{},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			vals := model.Values{
				"mykey": model.Value{
					Computed: &model.Computed{
						Template: tc.template,
					},
				},
			}
			cv := &ComputedValues{
				Env: map[string]any{},
			}
			target := map[string]any{}

			if err := Generate(vals, model.EnvironmentKindTenant, cv, target); err != nil {
				t.Fatalf("Generate returned error: %v", err)
			}

			if diff := cmp.Diff(tc.want, target); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGenerate_EmptyParentMapNotCreated(t *testing.T) {
	vals := model.Values{
		"parent.child": model.Value{
			Computed: &model.Computed{
				Template: "{{ .Env.missing_key }}",
			},
		},
	}
	cv := &ComputedValues{
		Env: map[string]any{},
	}
	target := map[string]any{}

	if err := Generate(vals, model.EnvironmentKindTenant, cv, target); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	if _, ok := target["parent"]; ok {
		t.Errorf("expected empty parent map to not be created, but got: %v", target)
	}
}

func TestGenerate_PresentValueNotOmitted(t *testing.T) {
	vals := model.Values{
		"org": model.Value{
			Computed: &model.Computed{
				Template: `{{ .Env.github_org | quote }}`,
			},
		},
	}
	cv := &ComputedValues{
		Env: map[string]any{
			"github_org": "navikt",
		},
	}
	target := map[string]any{}

	if err := Generate(vals, model.EnvironmentKindTenant, cv, target); err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	got, ok := target["org"]
	if !ok {
		t.Fatal("expected key 'org' to be present in target")
	}
	if got != "navikt" {
		t.Errorf("expected 'navikt', got %v", got)
	}
}
