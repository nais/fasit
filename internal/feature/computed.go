package feature

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"text/template"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature/featureutil"
	"gopkg.in/yaml.v3"
)

type ComputedConfig struct {
	DisplayName string `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
	Value       any    `yaml:"value,omitempty" json:"value,omitempty" jsonschema:"oneof_required=value"`
	Template    string `yaml:"template,omitempty" json:"template,omitempty" jsonschema:"oneof_required=template"`
}

type ComputedTenant struct {
	Name string
}

// ComputedFasit holds information about the Fasit instance managing the feature.
type ComputedFasit struct {
	// IAPAudience is the Identity-Aware Proxy audience of this Fasit instance,
	// e.g. so a feature like fasitd can authenticate calls back to Fasit.
	IAPAudience string
}

// fasitIAPAudience is the process-wide Fasit IAP audience exposed to feature
// templates via .Fasit.IAPAudience. It is set once at startup with
// SetIAPAudience before any rendering happens.
var fasitIAPAudience atomic.Value // string

// InitializeTemplateVars records the process-wide Fasit IAP audience surfaced to
// feature templates as .Fasit.IAPAudience.
func InitializeTemplateVars(aud string) {
	fasitIAPAudience.Store(aud)
}

// IAPAudience returns the process-wide Fasit IAP audience, or "" if unset.
func IAPAudience() string {
	aud, _ := fasitIAPAudience.Load().(string)
	return aud
}

type ComputedValues struct {
	// Kind is the kind of environment the feature is deployed to.
	Kind environment.EnvironmentKind
	// Tenant is information about the tenant that owns the cluster the feature is deployed to.
	Tenant ComputedTenant
	// Fasit is information about the Fasit instance managing the feature.
	Fasit ComputedFasit
	// Management is information about the management cluster for the tenant.
	Management map[string]any
	// Env contains information about the cluster the feature is deployed to.
	Env map[string]any
	// Envs contains information about all clusters the tenant has access to.
	Envs []map[string]any
	// Configs contains information about all configs stored on the feature.
	Configs map[string]any
}

func Generate(vals Values, kind environment.EnvironmentKind, values *ComputedValues, target map[string]any) error {
	return GenerateWith(vals, kind, values, target, TemplateFuncs)
}

func GenerateWith(vals Values, kind environment.EnvironmentKind, values *ComputedValues, target map[string]any, funcs template.FuncMap) error {
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

// RenderSingleTemplate renders a single Go template string against the given
// ComputedValues, returning the result as a string.
func RenderSingleTemplate(values *ComputedValues, tpl string) (string, error) {
	return renderString(values, tpl, TemplateFuncs)
}

// templateCache caches parsed templates keyed by (template string, func map pointer).
// The same feature version always produces the same template strings, so this
// avoids re-parsing+compiling on every render call.
var templateCache sync.Map // map[string]*template.Template

func renderString(values *ComputedValues, tpl string, funcs template.FuncMap) (string, error) {
	t, ok := templateCache.Load(tpl)
	if !ok {
		parsed, err := template.New("tpl").Funcs(funcs).Parse(tpl)
		if err != nil {
			return "", err
		}
		t, _ = templateCache.LoadOrStore(tpl, parsed)
	}

	buf := &bytes.Buffer{}
	if err := t.(*template.Template).Execute(buf, values); err != nil {
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

func ContainsKind(kinds []environment.EnvironmentKind, kind environment.EnvironmentKind) bool {
	return slices.Contains(kinds, kind)
}
