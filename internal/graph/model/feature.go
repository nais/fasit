package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/helm"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

// DownloadChartFunc is used to download a chart from a given source. It is a variable so that it can be mocked in tests.
var DownloadChartFunc = helm.DownloadChart

type Feature struct {
	FeatureYAML
	Name           string                     `json:"name"`
	Chart          string                     `json:"chart"`
	Version        string                     `json:"version"`
	Description    string                     `json:"description"`
	Source         string                     `json:"source"`
	ValuesYAML     map[string]json.RawMessage `json:"-"`
	HasDeployments bool                       `json:"hasDeployments"`

	// SpecVersion is used to determine which version of the feature spec is used.
	SpecVersion string `json:"specVersion"`

	// for graphql
	GraphVars struct {
		EnvironmentID uuid.UUID
		RolloutID     uuid.UUID
	} `json:"-" yaml:"-"`
}

type Rename struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type FeatureYAML struct {
	Dependencies     Dependencies      `json:"dependencies,omitempty" yaml:"dependencies,omitempty" jsonschema:"omitempty"`
	EnvironmentKinds []EnvironmentKind `json:"environmentKinds" yaml:"environmentKinds" jsonschema:"enum=management,enum=tenant,enum=onprem,enum=legacy,required"`
	Target           map[string]string `json:"target,omitempty" yaml:"target,omitempty" jsonschema:"omitempty,description=Target is a set of key-value label selectors; the feature applies only to environments whose labels include all specified pairs (logical AND)."`
	Timeout          time.Duration     `json:"timeout,omitempty" yaml:"timeout,omitempty" jsonschema:"omitempty,type=string,pattern=^(\\d+h)?(\\d+m)?(\\d+s)?$"`
	Values           Values            `json:"values,omitempty" yaml:"values,omitempty" jsonschema:"omitempty"`

	// Rename is a list of values that have been renamed to another key. This is an opperation that will be done when the rollout is created.
	// If the key is found, it will be renamed to the new key unless the new key already exists.
	// When this has been completely rolled out, it's safe to remove the rename statements.
	// It is only populated if read from a chart, or if the feature is a rollout.
	Rename []Rename `json:"rename,omitempty" yaml:"rename,omitempty" jsonschema:"omitempty"`
}

type Values map[string]Value

func (v *Values) Computed() map[string]Value {
	ret := map[string]Value{}
	for k, v := range *v {
		if v.Computed != nil {
			ret[k] = v
		}
	}
	return ret
}

type Computed struct {
	Template string `yaml:"template,omitempty" json:"template,omitempty"`
}

type Config struct {
	Type   ConfigType `yaml:"type,omitempty" json:"type"  jsonschema:"enum=string,enum=int,enum=bool,enum=string_array,required"`
	Secret bool       `json:"secret,omitempty" yaml:"secret,omitempty"`
}

type Value struct {
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	DisplayName string            `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Required    bool              `yaml:"required,omitempty" json:"required,omitempty"`
	Computed    *Computed         `yaml:"computed,omitempty" json:"computed,omitempty" jsonschema:"anyof_required=computed"`
	Config      *Config           `yaml:"config,omitempty" json:"config,omitempty" jsonschema:"anyof_required=config"`
	IgnoreKind  []EnvironmentKind `yaml:"ignoreKind,omitempty" json:"ignoreKind,omitempty" jsonschema:"enum=management,enum=tenant,enum=onprem,enum=legacy"`

	// for graphql
	GraphQLKey string `yaml:"key,omitempty" json:"key,omitempty" jsonschema:"-"`
}

func (v Value) ValidConfig(value json.RawMessage) error {
	if v.Config == nil {
		return fmt.Errorf("not configurable")
	}

	if v.Config.Type == "" {
		return fmt.Errorf("type is invalid")
	}

	var val any
	if err := json.Unmarshal(value, &val); err != nil {
		return fmt.Errorf("unable to decode json: %w", err)
	}

	switch val := val.(type) {
	case string:
		if v.Config.Type == ConfigTypeString {
			return nil
		}
	case int, int32, int64, float32, float64:
		if v.Config.Type == ConfigTypeInt {
			return nil
		}
	case bool:
		if v.Config.Type == ConfigTypeBool {
			return nil
		}
	case []any:
		if v.Config.Type == ConfigTypeStringArray {
			if !isStringArray(val) {
				return fmt.Errorf("array contains non-string elements")
			}
			return nil
		}
	}
	if val == nil {
		return nil
	}
	return fmt.Errorf("value doesn't match the required type. Expected %v, got %T", v.Config.Type, val)
}

func FromChart(chartURL, version string) (*Feature, error) {
	resp, err := DownloadChartFunc(chartURL, version, "")
	if err != nil {
		return nil, err
	}

	chart, err := loader.LoadArchive(resp)
	if err != nil {
		return nil, fmt.Errorf("unable to load chart: %w", err)
	}

	feat := &Feature{
		Chart: chartURL,
	}

	var hasFeatureYAML bool
	for _, f := range chart.Files {
		if f.Name == "Feature.yaml" {
			hasFeatureYAML = true
			if err := yaml.NewDecoder(bytes.NewReader(f.Data)).Decode(&feat.FeatureYAML); err != nil {
				return nil, err
			}
			break
		}
	}
	if !hasFeatureYAML {
		return nil, fmt.Errorf("file Feature.yaml not found")
	}

	if err := feat.parseChartYAML(chart.Metadata); err != nil {
		return nil, err
	}
	if err := chartutil.ProcessDependencies(chart, chart.Values); err != nil {
		return nil, fmt.Errorf("unable to process dependencies: %w", err)
	}

	feat.normalizedYAML(chart.Values)

	return feat, nil
}

func (f *Feature) parseChartYAML(meta *chart.Metadata) error {
	f.Name = meta.Name
	f.Version = meta.Version
	f.Description = meta.Description
	if len(meta.Sources) > 0 {
		f.Source = meta.Sources[0]
	}

	return nil
}

func (f *Feature) RequiredFields(envKind EnvironmentKind) []string {
	var requiredFields []string
	for k, v := range f.Values {
		if contains(v.IgnoreKind, envKind) {
			continue
		}
		if v.Required {
			requiredFields = append(requiredFields, k)
		}
	}
	return requiredFields
}

func (f *Feature) normalizedYAML(valuesYAML map[string]any) {
	if len(f.Values) == 0 || valuesYAML == nil {
		return
	}

	f.ValuesYAML = map[string]json.RawMessage{}
	for k, v := range f.Values {
		if v.Config == nil {
			continue
		}

		f.ValuesYAML[k] = pluckFromMap(k, valuesYAML)
	}
}

type FeatureHistory struct {
	ID           uuid.UUID          `json:"id"`
	Version      string             `json:"version"`
	Status       RolloutStatus      `json:"status"`
	Created      time.Time          `json:"created"`
	LastModified time.Time          `json:"lastModified"`
	Di           *DeployInstruction `json:"-"`
}

func pluckFromMap(key string, mp map[string]any) json.RawMessage {
	kp, _ := featureutil.SmartDotSplit(key)

	for _, k := range kp {
		v, ok := mp[k]
		if !ok {
			return nil
		}

		switch v := v.(type) {
		case map[string]any:
			mp = v
		default:
			b, _ := json.Marshal(v)
			return b
		}
	}
	return nil
}

func contains[T comparable](s []T, e T) bool {
	return slices.Contains(s, e)
}

// SecretKeys returns the dotted key names of all secret values in the feature.
func (f *Feature) SecretKeys() []string {
	var keys []string
	for key, val := range f.Values {
		if val.Config != nil && val.Config.Secret {
			keys = append(keys, key)
		}
	}
	return keys
}

func isStringArray(v []any) bool {
	for _, e := range v {
		if _, ok := e.(string); !ok {
			return false
		}
	}
	return true
}
