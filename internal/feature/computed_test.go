package feature

import (
	"testing"

	"github.com/nais/fasit/internal/graph/model"
)

func TestGenerate_MissingMappingValues(t *testing.T) {
	tests := map[string]struct {
		template string
		wantKey  bool
	}{
		"missing key without quote renders to <no value>": {
			template: "{{ .Env.missing_key }}",
			wantKey:  true, // produces "<no value>" string, not nil
		},
		"missing key with quote renders to nil": {
			template: `{{ .Env.missing_key | quote }}`,
			wantKey:  true, // quote("") -> "" -> yaml.Unmarshal -> nil, key still set
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

			_, ok := target["mykey"]
			if ok != tc.wantKey {
				t.Errorf("key present=%v, want %v; target=%v", ok, tc.wantKey, target)
			}
		})
	}
}

func TestGenerate_EmptyParentMapCreated(t *testing.T) {
	// In production, parent maps are created even when children resolve to nil.
	// Stripping of empty maps is handled by the playground resolver, not here.
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

	if _, ok := target["parent"]; !ok {
		t.Errorf("expected parent map to be created, got: %v", target)
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
