package fasit

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"github.com/sirupsen/logrus"
	"github.com/stevenle/topsort"
	"gopkg.in/yaml.v2"
)

func TestFeaturesSchema(t *testing.T) {
	schemaFile, err := os.OpenFile("schema/jsonschema/feature.json", os.O_RDONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema.json", schemaFile); err != nil {
		panic(err)
	}
	schema, err := compiler.Compile("schema.json")
	if err != nil {
		panic(err)
	}

	files := os.DirFS("./features")
	fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".yaml") {
			return nil
		}

		b, err := fs.ReadFile(files, path)
		if err != nil {
			return err
		}
		var m any
		err = yaml.Unmarshal(b, &m)
		if err != nil {
			t.Errorf("%v: %v", path, err)
			return nil
		}
		m, err = repairMapAny(m)
		if err != nil {
			t.Errorf("%v: %v", path, err)
			return nil
		}
		if err := schema.Validate(m); err != nil {
			t.Errorf("%v: %v", path, err)
			return nil
		}

		return nil
	})
}

func TestFeatures_MappingValuesTemplate(t *testing.T) {
	source, err := feature.NewFeatureSourceFilesystem("./features")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	mgr, err := feature.New(source, logrus.StandardLogger())
	if err != nil {
		t.Fatal(err)
	}

	for _, f := range mgr.Features() {
		t.Run(f.Name, func(t *testing.T) {
			for _, kind := range model.AllEnvironmentKind {
				mv := &feature.MappingValues{
					Kind: kind,
					Tenant: feature.MappingTenant{
						Name: "tenant_name",
					},
					Management: map[string]any{
						"key": "value",
					},
					Env: map[string]any{
						"name": "env",
						"is":   "current",
					},
					Envs: []map[string]any{
						{
							"name": "env1",
							"kind": "onprem",
							"is":   "not_current",
						},
						{
							"name": "env2",
							"kind": "onprem",
							"is":   "not_current",
						},
					},
				}
				target := map[string]any{}
				if err := f.Mapping.Generate(kind, mv, target); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestFeatures_dependencies(t *testing.T) {
	source, err := feature.NewFeatureSourceFilesystem("./features")
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()

	mgr, err := feature.New(source, logrus.StandardLogger())
	if err != nil {
		t.Fatal(err)
	}

	g := topsort.NewGraph()
	for _, f := range mgr.Features() {
		for _, deps := range f.DependsOn {
			for _, dep := range append(deps.AnyOf, deps.AllOf...) {
				g.AddEdge(f.Name, dep)
			}
		}
	}

	for _, f := range mgr.Features() {
		t.Run(f.Name, func(t *testing.T) {
			if len(f.EnvironmentKinds) == 0 {
				t.Error("no environment kind specified")
			}

			for _, kind := range f.EnvironmentKinds {
				if !kind.IsValid() {
					t.Errorf("invalid environment kind: %s", kind)
				}
			}

			if !g.ContainsNode(f.Name) {
				return
			}
			_, err := g.TopSort(f.Name)
			if err != nil {
				t.Errorf("Feature %s has a circular dependency: %v", f.Name, err)
			}
		})
	}
}

func repairMapAny(v any) (any, error) {
	var err error
	switch t := v.(type) {
	case []any:
		for i, v := range t {
			t[i], err = repairMapAny(v)
			if err != nil {
				return nil, err
			}
		}
	case map[any]any:
		nm := make(map[string]any)
		for k, v := range t {
			key, ok := k.(string)
			if !ok {
				return nil, fmt.Errorf("map key is not a string")
			}
			nm[key], err = repairMapAny(v)
			if err != nil {
				return nil, err
			}
		}
		return nm, nil
	}
	return v, nil
}
