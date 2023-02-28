package feature

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/pkg/graph/model"
)

func Test_mapOf(t *testing.T) {
	type args struct {
		m     []map[string]any
		key   string
		value string
	}
	tests := map[string]struct {
		args args
		want map[string]any
	}{
		"empty slice": {
			args: args{
				m:     []map[string]any{},
				key:   "foo",
				value: "bar",
			},
			want: map[string]any{},
		},
		"multiple slices": {
			args: args{
				m: []map[string]any{
					{
						"key": "bar",
						"val": "qux",
					},
					{
						"key": "baz",
						"val": "quux",
					},
					{
						"key": "ignored",
					},
				},
				key:   "key",
				value: "val",
			},
			want: map[string]any{
				"bar": "qux",
				"baz": "quux",
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := mapOf(tt.args.key, tt.args.value, tt.args.m); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_replace(t *testing.T) {
	type args struct {
		s   string
		old string
		new string
	}
	tests := map[string]struct {
		args args
		want string
	}{
		"empty string": {
			args: args{
				s:   "",
				old: "foo",
				new: "bar",
			},
			want: "",
		},
		"multiple replacements": {
			args: args{
				s:   "foo foo foo bar",
				old: "foo",
				new: "bar",
			},
			want: "bar bar bar bar",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := replace(tt.args.s, tt.args.old, tt.args.new); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_mapJoin(t *testing.T) {
	type args struct {
		m   any
		sep string
	}
	tests := map[string]struct {
		args args
		want []string
	}{
		"empty map": {
			args: args{
				m:   map[string]any{},
				sep: "=",
			},
			want: []string{},
		},
		"map with one entry": {
			args: args{
				m: map[string]any{
					"foo": "bar",
				},
				sep: "=",
			},
			want: []string{"foo=bar"},
		},
		"map with multiple entries": {
			args: args{
				m: map[string]any{
					"foo": "bar",
					"baz": "qux",
				},
				sep: "=",
			},
			want: []string{"baz=qux", "foo=bar"},
		},
		"map with string map": {
			args: args{
				m: map[string]string{
					"foo": "bar",
					"baz": "qux",
				},
				sep: "=",
			},
			want: []string{"baz=qux", "foo=bar"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := mapJoin(tt.args.sep, tt.args.m); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_prefixedValues(t *testing.T) {
	type args struct {
		m      any
		prefix string
	}
	tests := map[string]struct {
		args args
		want []any
	}{
		"empty map": {
			args: args{
				m:      map[string]any{},
				prefix: "foo",
			},
			want: []any{},
		},
		"map with matching key": {
			args: args{
				m: map[string]any{
					"foo": "bar",
				},
				prefix: "foo",
			},
			want: []any{"bar"},
		},
		"map with non-matching key": {
			args: args{
				m: map[string]any{
					"foo": "bar",
				},
				prefix: "baz",
			},
			want: []any{},
		},
		"map with matching key and non-matching key": {
			args: args{
				m: map[string]any{
					"foo": "bar",
					"baz": "qux",
				},
				prefix: "foo",
			},
			want: []any{"bar"},
		},
		"map with prefixes": {
			args: args{
				m: map[string]any{
					"foo":  "bar",
					"fbaz": "qux",
				},
				prefix: "f",
			},
			want: []any{"bar", "qux"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := prefixedValues(tt.args.m, tt.args.prefix); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_subdomain(t *testing.T) {
	type args struct {
		m      *MappingValues
		prefix string
	}
	tests := map[string]struct {
		args args
		want string
	}{
		"management": {
			args: args{
				m: &MappingValues{
					Kind: model.EnvironmentKindManagement,
					Tenant: MappingTenant{
						Name: "tenant",
					},
				},
				prefix: "foo",
			},
			want: "foo.tenant.cloud.nais.io",
		},
		"non-management": {
			args: args{
				m: &MappingValues{
					Kind: model.EnvironmentKindTenant,
					Env: map[string]any{
						"name": "bar",
					},
					Tenant: MappingTenant{
						Name: "baz",
					},
				},
				prefix: "foo",
			},
			want: "foo.bar.baz.cloud.nais.io",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := subdomain(tt.args.m, tt.args.prefix); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_eachOf(t *testing.T) {
	type testStruct struct {
		Foo string
	}

	type args struct {
		m   any
		key string
	}
	tests := map[string]struct {
		args args
		want []any
	}{
		"empty slice": {
			args: args{
				m:   []map[string]any{},
				key: "foo",
			},
			want: []any{},
		},
		"slice of map with matching key": {
			args: args{
				m: []map[string]any{
					{
						"foo": "bar",
					},
				},
				key: "foo",
			},
			want: []any{"bar"},
		},
		"slice of map with non-matching key": {
			args: args{
				m: []map[string]any{
					{
						"foo": "bar",
					},
				},
				key: "baz",
			},
			want: []any{},
		},

		"slice of map with matching key and non-matching key": {
			args: args{
				m: []map[string]any{
					{
						"foo": "bar",
					},
					{
						"baz": "qux",
					},
				},
				key: "foo",
			},
			want: []any{"bar"},
		},
		"slice of map with matching key and non-matching key and matching key": {
			args: args{
				m: []map[string]any{
					{
						"foo": "bar",
					},
					{
						"baz": "qux",
					},
					{
						"foo": "quux",
					},
				},
				key: "foo",
			},
			want: []any{"bar", "quux"},
		},

		"slice of struct with matching key": {
			args: args{
				m: []testStruct{
					{
						Foo: "bar",
					},
				},
				key: "Foo",
			},
			want: []any{"bar"},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := eachOf(tt.args.m, tt.args.key); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_toJSON(t *testing.T) {
	tests := map[string]struct {
		arg  any
		want string
	}{
		"empty map": {
			arg:  map[string]any{},
			want: "{}",
		},
		"map with one key": {
			arg: map[string]any{
				"foo": "bar",
			},
			want: `{"foo":"bar"}`,
		},
		"slice with one key": {
			arg: []string{
				"foo",
			},
			want: `["foo"]`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := toJSON(tt.arg); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_fromJSON(t *testing.T) {
	tests := map[string]struct {
		arg  string
		want map[string]any
	}{
		"empty map": {
			arg:  "{}",
			want: map[string]any{},
		},
		"map with one key": {
			arg: `{"foo":"bar"}`,
			want: map[string]any{
				"foo": "bar",
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := fromJSON(tt.arg); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_toYAML(t *testing.T) {
	tests := map[string]struct {
		arg  any
		want string
	}{
		"empty map": {
			arg:  map[string]any{},
			want: "{}\n",
		},
		"map with one key": {
			arg: map[string]any{
				"foo": "bar",
			},
			want: "foo: bar\n",
		},
		"slice with one key": {
			arg: []string{
				"foo",
			},
			want: "- foo\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := toYAML(tt.arg); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_usage(t *testing.T) {
	tests := map[string]struct {
		template string
		values   *MappingValues
		want     string
	}{
		"eachOf piped to toJSON": {
			template: `{{ eachOf .Envs "foo" | toJSON }}`,
			values: &MappingValues{
				Envs: []map[string]any{
					{
						"foo": "bar",
					},
					{
						"foo": "baz",
					},
				},
			},
			want: `["bar","baz"]`,
		},

		"mapOf piped to mapJoin piped to join": {
			template: `{{ mapOf "name" "project_id" .Envs | mapJoin "=" | join "," }}`,
			values: &MappingValues{
				Envs: []map[string]any{
					{
						"name":       "foo",
						"project_id": "bar",
					},
					{
						"name":       "baz",
						"project_id": "qux",
					},
				},
			},
			want: `baz=qux,foo=bar`,
		},

		"filter envs and mapOf piped to toJSON": {
			template: `{{ ( filter  "name" "foo" .Envs | mapOf "name" "project_id" ) | toJSON }}`,
			values: &MappingValues{
				Envs: []map[string]any{
					{
						"name":       "foo",
						"project_id": "bar",
					},
					{
						"name":       "baz",
						"project_id": "qux",
					},
				},
			},
			want: `{"foo":"bar"}`,
		},

		"map environment slice to a map keyed by cluster name": {
			template: `{{ (filter "kind" "tenant" .Envs | environmentsAsMap "value1,value2") | toJSON }}`,
			values: &MappingValues{
				Envs: []map[string]any{
					{
						"name":   "dev",
						"kind":   "tenant",
						"value1": "bar",
						"value2": "baz",
						"value3": "boo",
					},
					{
						"name":   "prod",
						"kind":   "tenant",
						"value1": "car",
						"value2": "caz",
						"value3": "coo",
					},
					{
						"name":   "onprem-prod",
						"kind":   "onprem",
						"value1": "car",
						"value2": "caz",
						"value3": "coo",
					},
				},
			},
			want: `{"dev":{"value1":"bar","value2":"baz"},"prod":{"value1":"car","value2":"caz"}}`,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			tpl := template.New("usage")
			tpl.Funcs(templateFuncs)
			tpl, err := tpl.Parse(tt.template)
			if err != nil {
				t.Fatalf("failed to parse template: %v", err)
			}

			var got bytes.Buffer
			if err := tpl.Execute(&got, tt.values); err != nil {
				t.Fatalf("failed to execute template: %v", err)
			}
			if !cmp.Equal(tt.want, got.String()) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got.String()))
			}
		})
	}
}

func Test_join(t *testing.T) {
	tests := map[string]struct {
		list []string
		sep  string
		want string
	}{
		"empty slice": {
			list: []string{},
			sep:  ",",
			want: "",
		},
		"slice with one element": {
			list: []string{"foo"},
			sep:  ",",
			want: "foo",
		},
		"slice with two elements": {
			list: []string{"foo", "bar"},
			sep:  ",",
			want: "foo,bar",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := join(tt.sep, tt.list); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_filter(t *testing.T) {
	type args struct {
		m    any
		key  string
		find any
	}
	tests := map[string]struct {
		args args
		want []map[string]any
	}{
		"empty map": {
			args: args{
				m:    []map[string]any{},
				key:  "foo",
				find: "bar",
			},
			want: []map[string]any{},
		},
		"map with one matching key": {
			args: args{
				m: []map[string]any{
					{"foo": "bar"},
				},
				key:  "foo",
				find: "bar",
			},
			want: []map[string]any{
				{"foo": "bar"},
			},
		},
		"map with two matching keys and one missing": {
			args: args{
				m: []map[string]any{
					{"foo": "bar", "baz": "qux"},
					{"foo": "bar", "baz": "quux"},
					{"bar": "qux"},
				},
				key:  "foo",
				find: "bar",
			},
			want: []map[string]any{
				{"foo": "bar", "baz": "qux"},
				{"foo": "bar", "baz": "quux"},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := filter(tt.args.key, tt.args.find, tt.args.m); !cmp.Equal(got, tt.want) {
				t.Errorf("diff -want +got:\n%v", cmp.Diff(tt.want, got))
			}
		})
	}
}

func Test_environmentsAsMap(t *testing.T) {
	input := &MappingValues{
		Envs: []map[string]any{
			{
				"name":   "dev",
				"value1": "bar",
				"value2": "baz",
				"value3": "boo",
			},
			{
				"name":   "prod",
				"value1": "car",
				"value2": "caz",
				"value3": "coo",
			},
		},
	}

	expectedOutput := map[string]map[string]any{
		"dev": {
			"value1": "bar",
			"value2": "baz",
		},
		"prod": {
			"value1": "car",
			"value2": "caz",
		},
	}

	output := environmentsAsMap("value1,value2", input.Envs)

	if !cmp.Equal(output, expectedOutput) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(expectedOutput, output))
	}
}
