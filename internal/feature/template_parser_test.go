package feature

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/nais/fasit/internal/graph/model"
)

func TestEnvValuesUsed(t *testing.T) {
	vals := model.Values{
		"a": {
			Computed: &model.Computed{
				Template: `{{ eachOf .Envs "name" | toJSON }}'`,
			},
		},
		"b": {
			Computed: &model.Computed{
				Template: `
{{with $root = .}}
	{{range $cluster := .Envs}}
	- name: {{ $root.Kind }}{{$cluster.name}}
	{{end}}
{{end}}
`,
			},
		},
		"c": {
			Computed: &model.Computed{
				Template: `
{{with $cn := .Env.name}}
	{{if $cn}}
		{{.Management.some_value}} {{$cn}}
	{{end}}
{{end}}
`,
			},
		},
		"d": {
			Computed: &model.Computed{
				Template: `
{{ .Env.name }}
{{ .Env.value | quote }}
`,
			},
		},
	}

	got, err := ParseTemplateDetails(vals)
	if err != nil {
		t.Fatal(err)
	}

	want := &FeatureTemplateDetails{
		Management: []string{
			"some_value",
		},
		Env: []string{
			"name",
			"value",
		},
		Envs: []string{
			"name",
		},
		All: []string{
			"$cn",
			"Env.name",
			"Env.value",
			"Envs",
			"Envs.name",
			"Kind",
			"Management.some_value",
		},
		Functions: []string{
			"eachOf",
			"quote",
			"toJSON",
		},
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}
