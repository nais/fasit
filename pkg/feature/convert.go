package feature

import (
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/graph/model"
)

func FeatureV2(f *Feature, debug bool) *model.FeatureYAML {
	fy := &model.FeatureYAML{
		EnvironmentKinds: f.EnvironmentKinds,
		Timeout:          f.Timeout,
		Values:           make(model.Values),
	}
	db, _ := json.Marshal(f.DependsOn)
	json.Unmarshal(db, &fy.Dependencies)

	for k, c := range f.Config {
		fy.Values[k] = model.Value{
			Description: c.Description,
			DisplayName: c.DisplayName,
			Required:    c.Required,
			IgnoreKind:  c.IgnoreKind,
			Config: &model.Config{
				Type:   c.Type,
				Secret: c.Secret,
			},
		}
	}

	for k, c := range f.Mapping {
		t, ok := fy.Values[k]

		if debug && len(t.IgnoreKind) > 0 && len(c.IgnoreKind) > 0 {
			fmt.Printf("DOUBLE CHECK: %v - %v\n", f.Name, k)
		}
		if !ok {
			t = model.Value{
				Description: c.Description,
				DisplayName: c.DisplayName,
				IgnoreKind:  c.IgnoreKind,
			}
		}

		tpl := c.Template
		if tpl == "" {
			switch t := c.Value.(type) {
			case string:
				tpl = `"` + t + `"`
			default:
				panic(fmt.Errorf("unsupported mapping type for %q: %T", k, t))
			}
		}
		t.Computed = &model.Computed{
			Template: tpl,
		}

		fy.Values[k] = t
	}

	if len(f.AutoInstall) > 0 && debug {
		fmt.Println("WARNING: 'autoInstall' is not supported in v2")
	}

	return fy
}
