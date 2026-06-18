package feature

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/helm"
	"gopkg.in/yaml.v3"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
)

// MergedConfigRow is a config key-value pair produced by overlaying
// environment-specific config on top of global config. Exported so that
// the reconciler package can build these from its own bulk-fetched data.
type MergedConfigRow struct {
	ID            uuid.UUID
	Key           string
	Value         []byte
	Secret        bool
	Created       time.Time
	EnvironmentID *uuid.UUID
}

func MergeConfigs(global []featuresql.ConfigurationsGlobal, env []featuresql.ConfigurationsEnvironment, includeKeys []string) []MergedConfigRow {
	keySet := make(map[string]struct{}, len(includeKeys))
	for _, k := range includeKeys {
		keySet[k] = struct{}{}
	}

	m := make(map[string]MergedConfigRow, len(global)+len(env))
	for _, g := range global {
		if len(keySet) > 0 {
			if _, ok := keySet[g.Key]; !ok {
				continue
			}
		}
		m[g.Key] = MergedConfigRow{ID: g.ID, Key: g.Key, Value: g.Value, Secret: g.Secret, Created: g.Created}
	}
	for _, e := range env {
		if len(keySet) > 0 {
			if _, ok := keySet[e.Key]; !ok {
				continue
			}
		}
		eid := e.EnvironmentID
		m[e.Key] = MergedConfigRow{ID: e.ID, Key: e.Key, Value: e.Value, Secret: e.Secret, Created: e.Created, EnvironmentID: &eid}
	}

	result := make([]MergedConfigRow, 0, len(m))
	for _, v := range m {
		result = append(result, v)
	}
	// Sort by key for deterministic iteration order downstream.
	slices.SortFunc(result, func(a, b MergedConfigRow) int {
		return cmp.Compare(a.Key, b.Key)
	})
	return result
}

func MakeHelmConfigMap(vals []MergedConfigRow) (map[string]any, error) {
	// Pre-scan: a leaf key cannot also be a strict dotted prefix of another key.
	leaves := make(map[string]bool, len(vals))
	for _, v := range vals {
		leaves[v.Key] = true
	}
	for _, v := range vals {
		keys, err := featureutil.SmartDotSplit(v.Key)
		if err != nil {
			return nil, err
		}
		for i := 1; i < len(keys); i++ {
			prefix := strings.Join(keys[:i], ".")
			if leaves[prefix] {
				return nil, fmt.Errorf("key %v is not nestable", prefix)
			}
		}
	}

	val := make(map[string]any)
	for _, v := range vals {
		keys, err := featureutil.SmartDotSplit(v.Key)
		if err != nil {
			return nil, err
		}
		parent := val
		for index, key := range keys {
			if index == len(keys)-1 {
				parent[key] = json.RawMessage(v.Value)
				continue
			}
			if e, ok := parent[key]; ok {
				parent = e.(map[string]any)
				continue
			}
			f := make(map[string]any)
			parent[key] = f
			parent = f
		}
	}
	return val, nil
}

func ValidateFields(f *Feature, envKind environment.EnvironmentKind, values []MergedConfigRow, mp map[string]any) []string {
	requiredFields := f.RequiredFields(envKind)

	fields := map[string]bool{}
	for _, req := range requiredFields {
		fields[req] = false
		for _, k := range values {
			if k.Key == req {
				fields[req] = true
			}
		}
	}

	var missing []string
	for field, present := range fields {
		if present {
			continue
		}

		parts, _ := featureutil.SmartDotSplit(field)
		parent := mp
		for i, part := range parts {
			v, ok := parent[part]
			// A required field whose only value is nil (e.g. a computed
			// template that rendered to nothing) is not considered set.
			// An explicitly provided empty string lives in values above and
			// is already treated as present.
			if !ok || v == nil {
				missing = append(missing, field)
				break
			}
			if i == len(parts)-1 {
				break
			}
			p, ok := v.(map[string]any)
			if !ok {
				continue
			}
			parent = p
		}
	}
	slices.Sort(missing)
	return missing
}

func environmentKindToSQL(kinds []environment.EnvironmentKind) []string {
	ret := []string{}
	for _, kind := range kinds {
		ret = append(ret, kind.String())
	}
	slices.Sort(ret)
	return ret
}

func featureFromSQL(f featuresql.FeatureDatum) (*Feature, error) {
	fyaml, defaultValues, err := makeFeatureYAML(f)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &Feature{
		FeatureYAML: fyaml,
		Name:        f.Name,
		Chart:       f.Chart,
		Version:     f.Version,
		Description: f.Description,
		Source:      f.Source,
		ValuesYAML:  defaultValues,
		SpecVersion: "v2",
		TplDetails:  f.TplDetails,
	}, nil
}

func makeFeatureYAML(fd featuresql.FeatureDatum) (FeatureYAML, map[string]json.RawMessage, error) {
	ret := FeatureYAML{
		Timeout: time.Duration(fd.Timeout) * time.Millisecond,
	}
	if err := json.Unmarshal(fd.Dependencies, &ret.Dependencies); err != nil {
		return ret, nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	var retDefaultVals map[string]json.RawMessage
	if err := json.Unmarshal(fd.DefaultValues, &retDefaultVals); err != nil {
		return ret, nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	ret.EnvironmentKinds = make([]environment.EnvironmentKind, len(fd.Kinds))
	for i, k := range fd.Kinds {
		ret.EnvironmentKinds[i] = environment.EnvironmentKind(k)
	}

	if err := json.Unmarshal(fd.Values, &ret.Values); err != nil {
		return ret, nil, fmt.Errorf("unmarshal values: %w", err)
	}

	return ret, retDefaultVals, nil
}

type Feature struct {
	FeatureYAML
	Name        string                     `json:"name"`
	Chart       string                     `json:"chart"`
	Version     string                     `json:"version"`
	Description string                     `json:"description"`
	Source      string                     `json:"source"`
	ValuesYAML  map[string]json.RawMessage `json:"-"`

	// SpecVersion is used to determine which version of the feature spec is used.
	SpecVersion string `json:"specVersion"`
	TplDetails  []byte
}

type FeatureYAML struct {
	Dependencies     Dependencies                  `json:"dependencies,omitempty" yaml:"dependencies,omitempty" jsonschema:"omitempty"`
	EnvironmentKinds []environment.EnvironmentKind `json:"environmentKinds" yaml:"environmentKinds" jsonschema:"enum=management,enum=tenant,enum=onprem,required"`
	Timeout          time.Duration                 `json:"timeout,omitempty" yaml:"timeout,omitempty" jsonschema:"omitempty,type=string,pattern=^(\\d+h)?(\\d+m)?(\\d+s)?$"`
	Values           Values                        `json:"values,omitempty" yaml:"values,omitempty" jsonschema:"omitempty"`
}

type Values map[string]Value

type Value struct {
	Description string                        `yaml:"description,omitempty" json:"description,omitempty"`
	DisplayName string                        `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	Required    bool                          `yaml:"required,omitempty" json:"required,omitempty"`
	Computed    *Computed                     `yaml:"computed,omitempty" json:"computed,omitempty" jsonschema:"anyof_required=computed"`
	Config      *Config                       `yaml:"config,omitempty" json:"config,omitempty" jsonschema:"anyof_required=config"`
	IgnoreKind  []environment.EnvironmentKind `yaml:"ignoreKind,omitempty" json:"ignoreKind,omitempty" jsonschema:"enum=management,enum=tenant,enum=onprem"`
}

func (v *Values) Computed() map[string]Value {
	ret := map[string]Value{}
	for k, v := range *v {
		if v.Computed != nil {
			ret[k] = v
		}
	}
	return ret
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

func (f *Feature) RequiredFields(envKind environment.EnvironmentKind) []string {
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

// DownloadChartFunc is used to download a chart from a given source. It is a variable so that it can be mocked in tests.
var DownloadChartFunc = helm.DownloadChart

type Config struct {
	Type   ConfigType `yaml:"type,omitempty" json:"type"  jsonschema:"enum=string,enum=int,enum=bool,enum=string_array,required"`
	Secret bool       `json:"secret,omitempty" yaml:"secret,omitempty"`
}

type Computed struct {
	Template string `yaml:"template,omitempty" json:"template,omitempty"`
}

type Dependency struct {
	AnyOf []string `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf []string `json:"allOf,omitempty" yaml:"allOf,omitempty"`
}

type Dependencies []*Dependency

func (d Dependencies) FindMissing(features []string) []string {
	ret := []string{}
	for _, dep := range d {
		ret = append(ret, dep.FindMissing(features)...)
	}
	return ret
}

func (d *Dependency) FindMissing(features []string) []string {
	contains := func(s []string, e string) bool {
		return slices.Contains(s, e)
	}

	missing := []string{}
	if len(d.AllOf) > 0 {
		for _, f := range d.AllOf {
			if !contains(features, f) {
				missing = append(missing, f)
			}
		}
	}

	anyOfMissing := []string{}
	for _, f := range d.AnyOf {
		if contains(features, f) {
			anyOfMissing = []string{}
			break
		}
		anyOfMissing = append(anyOfMissing, f)
	}
	return append(missing, anyOfMissing...)
}

type NewConfiguration struct {
	EnvironmentID *uuid.UUID      `json:"environmentID"`
	Feature       string          `json:"feature"`
	Description   *string         `json:"description"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	Secret        bool
}

type ConfigType string

const (
	ConfigTypeString      ConfigType = "string"
	ConfigTypeInt         ConfigType = "int"
	ConfigTypeBool        ConfigType = "bool"
	ConfigTypeStringArray ConfigType = "string_array"
)

func (e ConfigType) IsValid() bool {
	switch e {
	case ConfigTypeString, ConfigTypeInt, ConfigTypeBool, ConfigTypeStringArray:
		return true
	}
	return false
}

func (e ConfigType) String() string {
	return string(e)
}

func (e ConfigType) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToUpper(string(e)))
}

func (e *ConfigType) UnmarshalJSON(b []byte) error {
	s := ""
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*e = ConfigType(strings.ToLower(s))
	return nil
}

type Configuration struct {
	ID      uuid.UUID       `json:"id"`
	Value   *Value          `json:"value"`
	Content json.RawMessage `json:"content"`
	Created time.Time       `json:"created"`
	Source  ConfigSource    `json:"source"`
	Key     string          `json:"key"`
}

type DeployInstruction struct {
	ID                  uuid.UUID
	EnvironmentID       uuid.UUID
	FeatureAssignmentID *uuid.UUID
	FeatureName         string
	FeatureVersion      string
	Status              DeployStatus
	Hash                string
	Created             time.Time
	LastModified        time.Time

	// Helm values for this deploy instruction.
	Values []byte
}

type UpdateConfiguration struct {
	Description *string         `json:"description,omitempty"`
	Value       json.RawMessage `json:"value"`
}

type ConfigSource string

const (
	ConfigSourceGlobal  ConfigSource = "GLOBAL"
	ConfigSourceEnv     ConfigSource = "ENV"
	ConfigSourceHelm    ConfigSource = "HELM"
	ConfigSourceUnknown ConfigSource = "UNKNOWN"
)

func (e ConfigSource) IsValid() bool {
	switch e {
	case ConfigSourceGlobal, ConfigSourceEnv, ConfigSourceHelm, ConfigSourceUnknown:
		return true
	}
	return false
}

func (e ConfigSource) String() string {
	return string(e)
}

type Release struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	Status       string    `json:"status"`
	Revision     int       `json:"revision"`
	LastDeployed time.Time `json:"lastDeployed"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
}

type DeployStatus string

const (
	DeployStatusUnknown    DeployStatus = ""
	DeployStatusSent       DeployStatus = "sent"
	DeployStatusInstalling DeployStatus = "installing"
	DeployStatusDeployed   DeployStatus = "deployed"
	DeployStatusFailed     DeployStatus = "failed"
)

func (r DeployStatus) IsValid() bool {
	switch r {
	case DeployStatusUnknown, DeployStatusSent, DeployStatusInstalling, DeployStatusDeployed, DeployStatusFailed:
		return true
	}
	return false
}

// IsInProgress reports whether the rollout has been dispatched but has not yet
// reached a terminal state (deployed/failed).
func (r DeployStatus) IsInProgress() bool {
	return r == DeployStatusSent || r == DeployStatusInstalling
}

func (r DeployStatus) String() string {
	return string(r)
}
