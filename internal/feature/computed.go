package feature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"text/template"

	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
	"gopkg.in/yaml.v3"
)

type Computed map[string]ComputedConfig

type ComputedConfig struct {
	DisplayName string `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Value       any    `yaml:"value,omitempty" json:"value,omitempty" jsonschema:"oneof_required=value"`
	Template    string `yaml:"template,omitempty" json:"template,omitempty" jsonschema:"oneof_required=template"`
}

type ComputedTenant struct {
	Name string
}

type ComputedValues struct {
	// Kind is the kind of environment the feature is deployed to.
	Kind model.EnvironmentKind
	// Tenant is information about the tenant that owns the cluster the feature is deployed to.
	Tenant ComputedTenant
	// Management is information about the management cluster for the tenant.
	Management map[string]any
	// Env contains information about the cluster the feature is deployed to.
	Env map[string]any
	// Envs contains information about all clusters the tenant has access to.
	Envs []map[string]any
	// Configs contains information about all configs stored on the feature.
	Configs map[string]any
}

func Generate(vals model.Values, kind model.EnvironmentKind, values *ComputedValues, target map[string]any) error {
	return GenerateWith(vals, kind, values, target, templateFuncs)
}

func GenerateWith(vals model.Values, kind model.EnvironmentKind, values *ComputedValues, target map[string]any, funcs template.FuncMap) error {
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	if values == nil {
		return fmt.Errorf("values is nil")
	}

	values.Configs = copyMap(target)

	for k, v := range vals {
		if v.Computed == nil {
			continue
		}
		if ContainsKind(v.IgnoreKind, kind) {
			continue
		}
		keys, err := featureutil.SmartDotSplit(k)
		if err != nil {
			return err
		}

		if err := addToMap(target, values, keys, v.Computed.Template, funcs); err != nil {
			return err
		}
	}
	return nil
}

func addToMap(target map[string]any, values *ComputedValues, key []string, v string, funcs template.FuncMap) error {
	if len(key) > 1 {
		t, ok := target[key[0]]
		if !ok {
			t = make(map[string]any)
			target[key[0]] = t
		}
		tt, ok := t.(map[string]any)
		if !ok {
			return fmt.Errorf("key %v is not nestable", key[0])
		}
		return addToMap(tt, values, key[1:], v, funcs)
	}

	val, err := renderTemplate(values, v, funcs)
	if err != nil {
		return fmt.Errorf("%v: %w", strings.Join(key, "."), err)
	}

	if _, ok := target[key[0]]; ok {
		return nil
	}

	target[key[0]] = val
	return nil
}

func renderTemplate(values *ComputedValues, tpl string, funcs template.FuncMap) (any, error) {
	if tpl == "" {
		return nil, fmt.Errorf("empty template")
	}
	rdr, err := renderString(values, tpl, funcs)
	if err != nil {
		return nil, err
	}

	var v any
	if err := yaml.Unmarshal([]byte(rdr), &v); err != nil {
		return nil, err
	}

	v = repairMapAny(v)

	return v, nil
}

func renderString(values *ComputedValues, tpl string, funcs template.FuncMap) (string, error) {
	t := template.New("tpl")
	t.Funcs(funcs)
	t, err := t.Parse(tpl)
	if err != nil {
		return "", err
	}

	buf := &bytes.Buffer{}
	if err := t.Execute(buf, values); err != nil {
		return "", err
	}

	return buf.String(), nil
}

func repairMapAny(v any) any {
	switch t := v.(type) {
	case []any:
		for i, v := range t {
			t[i] = repairMapAny(v)
		}
	case map[any]any:
		nm := make(map[string]any)
		for k, v := range t {
			nm[k.(string)] = repairMapAny(v)
		}
		return nm
	}
	return v
}

func copyMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}
	ret := map[string]any{}
	b, _ := json.Marshal(m)
	_ = json.Unmarshal(b, &ret)
	return ret
}

func ContainsKind(kinds []model.EnvironmentKind, kind model.EnvironmentKind) bool {
	return slices.Contains(kinds, kind)
}
