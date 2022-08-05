package feature

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRenderString(t *testing.T) {
	tests := map[string]struct {
		values   *MappingValues
		input    string
		secret   bool
		expected string
		err      error
	}{
		"empty": {
			input:    "",
			expected: "",
		},
		"simple": {
			values:   &MappingValues{Tenant: MappingTenant{Name: "bar"}},
			input:    "{{.Tenant.Name}}",
			expected: "bar",
		},
		"secret": {
			values:   &MappingValues{Tenant: MappingTenant{Name: "bar"}},
			input:    "{{.Tenant.Name}}",
			expected: "***",
			secret:   true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			actual, err := renderString(tc.values, tc.input, tc.secret)
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
	tests := map[string]struct {
		mapping     Mapping
		values      *MappingValues
		target      map[string]any
		expected    map[string]any
		hideSecrets bool
		err         error
	}{
		"empty": {
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
		"single_level_with_secret": {
			values: &MappingValues{Tenant: MappingTenant{Name: "foo"}},
			mapping: Mapping{
				"foo": MappingConfig{
					DisplayName: "Tenant name",
					Value:       "{{.Tenant.Name}}",
				},
				"bar": MappingConfig{
					DisplayName: "Tenant name",
					Value:       "{{.Tenant.Name}}",
					Secret:      true,
				},
			},
			target:      map[string]any{},
			expected:    map[string]any{"foo": "foo", "bar": "***"},
			hideSecrets: true,
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
			err := tc.mapping.Generate(tc.values, tc.target, tc.hideSecrets)
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

func TestSmartDotSplit(t *testing.T) {
	tests := map[string]struct {
		input    string
		expected []string
		err      error
	}{
		"empty": {
			input:    "",
			expected: []string{""},
		},
		"single_level": {
			input:    "test1",
			expected: []string{"test1"},
		},
		"multi_level": {
			input:    "test.a",
			expected: []string{"test", "a"},
		},
		"escaped dots": {
			input:    "test\\.a",
			expected: []string{"test.a"},
		},
		"end with .": {
			input: "test.a.",
			err:   errors.New("cannot end with `.`"),
		},
		"starts with .": {
			input: ".test.a",
			err:   errors.New("cannot start with `.`"),
		},
		"contains ..": {
			input: "test..a",
			err:   errors.New("invalid `.` on position 5"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			retval, err := SmartDotSplit(tc.input)
			if err != nil {
				if tc.err == nil {
					t.Fatal(err)
				}
				if tc.err.Error() != err.Error() {
					t.Errorf("got %q, want %q", err, tc.err)
				}
			} else if tc.err != nil {
				t.Errorf("got nil, want %q", tc.err)
			}
			if !cmp.Equal(retval, tc.expected) {
				t.Error(cmp.Diff(retval, tc.expected))
			}
		})
	}
}
