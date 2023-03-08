package feature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"text/template"

	"github.com/nais/fasit/pkg/graph/model"
	"gopkg.in/yaml.v2"
)

type Mapping map[string]MappingConfig

type MappingConfig struct {
	DisplayName string      `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Description string      `yaml:"description,omitempty" json:"description,omitempty"`
	Value       any         `yaml:"value,omitempty" json:"value,omitempty" jsonschema:"oneof_required=value"`
	Template    string      `yaml:"template,omitempty" json:"template,omitempty" jsonschema:"oneof_required=template"`
	IgnoreKind  IgnoreKinds `yaml:"ignoreKind,omitempty" json:"ignoreKind,omitempty"`
}

type MappingTenant struct {
	Name string
}

type MappingValues struct {
	// Kind is the kind of environment the feature is deployed to.
	Kind model.EnvironmentKind
	// Tenant is information about the tenant that owns the cluster the feature is deployed to.
	Tenant MappingTenant
	// Management is information about the management cluster for the tenant.
	Management map[string]any
	// Env contains information about the cluster the feature is deployed to.
	Env map[string]any
	// Envs contains information about all clusters the tenant has access to.
	Envs []map[string]any
	// Configs contains information about all configs stored on the feature.
	Configs map[string]any
}

func (m Mapping) Generate(envKind model.EnvironmentKind, values *MappingValues, target map[string]any) error {
	if target == nil {
		return fmt.Errorf("target is nil")
	}
	if values == nil {
		return fmt.Errorf("values is nil")
	}

	values.Configs = copyMap(target)

	for k, v := range m {
		keys, err := SmartDotSplit(k)
		if err != nil {
			return err
		}

		if v.IgnoreKind.Contains(envKind) {
			continue
		}

		if err := addToMap(target, values, keys, v, k); err != nil {
			return err
		}
	}
	return nil
}

func (m Mapping) GenerateJSON(envKind model.EnvironmentKind, values *MappingValues) (string, error) {
	target := make(map[string]any)
	if err := m.Generate(envKind, values, target); err != nil {
		return "", err
	}
	return "", nil
}

func (m Mapping) DisplayName(key string) string {
	for k, v := range m {
		if k == key {
			return v.DisplayName
		}
	}

	return ""
}

func addToMap(target map[string]any, values *MappingValues, key []string, mc MappingConfig, path string) error {
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
		return addToMap(tt, values, key[1:], mc, path)
	}

	val, err := renderTpl(values, mc)
	if err != nil {
		return fmt.Errorf("%v: %w", strings.Join(key, "."), err)
	}

	if _, ok := target[key[0]]; ok {
		return nil
	}

	target[key[0]] = val
	return nil
}

func renderTpl(values *MappingValues, mc MappingConfig) (any, error) {
	switch t := mc.Value.(type) {
	case string:
		return renderString(values, t)
	case []any:
		return renderSlice(values, t)
	default:
		if mc.Template != "" {
			return renderTemplate(values, mc.Template)
		}
		return nil, fmt.Errorf("unsupported type %T", t)
	}
}

func renderTemplate(values *MappingValues, tpl string) (any, error) {
	rdr, err := renderString(values, tpl)
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

func renderString(values *MappingValues, tpl string) (string, error) {
	t := template.New("tpl")
	t.Funcs(templateFuncs)
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

func renderSlice(values *MappingValues, tpl []any) ([]any, error) {
	ret := make([]any, len(tpl))
	for i, t := range tpl {
		val, err := renderTpl(values, MappingConfig{
			Value: t,
		})
		if err != nil {
			return nil, err
		}
		ret[i] = val
	}
	return ret, nil
}

func SmartDotSplit(s string) ([]string, error) {
	if strings.HasSuffix(s, ".") {
		return nil, fmt.Errorf("cannot end with `.`")
	}
	if strings.HasPrefix(s, ".") {
		return nil, fmt.Errorf("cannot start with `.`")
	}

	str := ""
	var ret []string
	for i, ch := range s {
		switch ch {
		case '.':
			if len(str) == 0 || i == 0 {
				return nil, fmt.Errorf("invalid `.` on position %v", i)
			}
			if s[i-1] == '\\' {
				str = str[:len(str)-1]
				str += "."
			} else {
				ret = append(ret, str)
				str = ""
			}
		default:
			str += string(ch)
		}
	}
	return append(ret, str), nil
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
