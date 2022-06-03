package feature

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/nais/fasit/pkg/graph/model"
)

type ErrOverride struct {
	Path string
}

func (e ErrOverride) Error() string {
	return fmt.Sprintf("override error: %v", e.Path)
}

func (e ErrOverride) Is(err error) bool {
	t, ok := err.(*ErrOverride)
	if !ok {
		return false
	}

	return t.Path == e.Path
}

type Mapping map[string]MappingConfig

type MappingConfig struct {
	DisplayName string `yaml:"displayName,omitempty"`
	Value       any    `yaml:"value"`
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
}

func (m Mapping) Generate(values *MappingValues, target map[string]any) error {
	if target == nil {
		return fmt.Errorf("target is nil")
	}

	for k, v := range m {
		keys, err := SmartDotSplit(k)
		if err != nil {
			return err
		}

		if err := addToMap(target, values, keys, v.Value, k); err != nil {
			return err
		}
	}
	return nil
}

func (m Mapping) DisplayName(key string) string {
	for k, v := range m {
		if k == key {
			return v.DisplayName
		}
	}

	return ""
}

func addToMap(target map[string]any, values *MappingValues, key []string, tpl any, path string) error {
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
		return addToMap(tt, values, key[1:], tpl, path)
	}

	val, err := renderTpl(values, tpl)
	if err != nil {
		return err
	}

	if _, ok := target[key[0]]; ok {
		return &ErrOverride{Path: path}
	}

	target[key[0]] = val
	return nil
}

func renderTpl(values *MappingValues, tpl any) (any, error) {
	switch t := tpl.(type) {
	case string:
		return renderString(values, t)
	case []any:
		return renderSlice(values, t)
	default:
		return nil, fmt.Errorf("unsupported type %T", t)
	}
}

func renderString(values *MappingValues, tpl string) (string, error) {
	t, err := template.New("tpl").Parse(tpl)
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
		val, err := renderTpl(values, t)
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
