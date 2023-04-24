package feature

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/graph/model"
)

func TestRenderString(t *testing.T) {
	tests := map[string]struct {
		values   *MappingValues
		input    string
		expected string
		err      error
	}{
		"empty": {
			values:   &MappingValues{},
			input:    "",
			expected: "",
		},
		"simple": {
			values:   &MappingValues{Tenant: MappingTenant{Name: "bar"}},
			input:    "{{.Tenant.Name}}",
			expected: "bar",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := renderString(tc.values, tc.input)
			if diff := cmp.Diff(tc.err, err, cmp.Comparer(errors.Is)); diff != "" {
				t.Errorf("renderString(%v) mismatch (-want +got):\n%s", tc.input, diff)
			}
			if diff := cmp.Diff(tc.expected, actual); diff != "" {
				t.Errorf("renderString(%v) mismatch (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func TestMapping_Generate(t *testing.T) {
	// All tests are run as "tenant" environment if not defined otherwise.
	tests := map[string]struct {
		mapping  Mapping
		values   *MappingValues
		target   map[string]any
		expected map[string]any
		envKind  model.EnvironmentKind
		err      error
	}{
		"empty": {
			values:   &MappingValues{},
			target:   map[string]any{},
			expected: map[string]any{},
		},
		"single_level": {
			values: &MappingValues{Tenant: MappingTenant{Name: "foo"}},
			mapping: Mapping{
				"foo": MappingConfig{
					DisplayName: "Tenant name",
					Value:       "{{.Tenant.Name}}",
				},
			},
			target:   map[string]any{},
			expected: map[string]any{"foo": "foo"},
		},
		"single_level_with_ignore": {
			values: &MappingValues{Tenant: MappingTenant{Name: "foo"}},
			mapping: Mapping{
				"foo": MappingConfig{
					DisplayName: "Tenant name",
					Value:       "{{.Tenant.Name}}",
					IgnoreKind:  []model.EnvironmentKind{model.EnvironmentKindTenant},
				},
			},
			target:   map[string]any{},
			expected: map[string]any{},
		},
		"multi_level": {
			values: &MappingValues{Tenant: MappingTenant{Name: "foo"}, Management: map[string]any{"project_id": "gcp"}},
			mapping: Mapping{
				"foo.name": MappingConfig{
					Value: "{{.Tenant.Name}}",
				},
				"foo.project": MappingConfig{
					Value: "{{.Management.project_id}}",
				},
			},
			target:   map[string]any{},
			expected: map[string]any{"foo": map[string]any{"name": "foo", "project": "gcp"}},
		},
		"multi_level_with_existing": {
			values: &MappingValues{Tenant: MappingTenant{Name: "foo"}, Management: map[string]any{"project_id": "gcp"}},
			mapping: Mapping{
				"foo.project": MappingConfig{
					Value: "{{.Management.project_id}}",
				},
			},
			target:   map[string]any{"foo": map[string]any{"name": "foo"}},
			expected: map[string]any{"foo": map[string]any{"name": "foo", "project": "gcp"}},
		},
		"multi_level_with_existing_nested": {
			values: &MappingValues{Tenant: MappingTenant{Name: "foo"}, Management: map[string]any{"project_id": "gcp"}},
			mapping: Mapping{
				"foo.project": MappingConfig{
					Value: "{{.Management.project_id}}",
				},
			},
			target:   map[string]any{"foo": map[string]any{"name": "foo", "project": "bar"}},
			expected: map[string]any{"foo": map[string]any{"name": "foo", "project": "bar"}},
		},
		"template_object_array": {
			values: &MappingValues{
				Tenant:     MappingTenant{Name: "foo"},
				Management: map[string]any{"project_id": "gcp"},
				Envs:       []map[string]any{{"name": "dev"}, {"name": "prod"}},
			},
			mapping: Mapping{
				"foo.project": MappingConfig{
					Template: `{{ range $env := .Envs }}
- name: {{ $env.name }}
{{end}}`,
				},
			},
			target: map[string]any{},
			expected: map[string]any{
				"foo": map[string]any{
					"project": []any{
						map[string]any{"name": "dev"},
						map[string]any{"name": "prod"},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			kind := model.EnvironmentKindTenant
			if tc.envKind != "" {
				kind = tc.envKind
			}
			err := tc.mapping.Generate(kind, tc.values, tc.target)
			if diff := cmp.Diff(tc.err, err, cmp.Comparer(errors.Is)); diff != "" {
				t.Errorf("Generate(%v) mismatch (-want +got):\n%s", tc.values, diff)
			}

			if tc.err == nil {
				if diff := cmp.Diff(tc.expected, tc.target); diff != "" {
					t.Errorf("Generate(%v) mismatch (-want +got):\n%s", tc.values, diff)
				}
			}
		})
	}
}
