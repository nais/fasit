package feature

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
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

func ValidateFields(f *model.Feature, envKind model.EnvironmentKind, values []MergedConfigRow, mp map[string]any) []string {
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
		for _, part := range parts {
			if p, ok := parent[part].(map[string]any); ok {
				parent = p
				continue
			}
			if _, ok := parent[part]; ok {
				continue
			}
			missing = append(missing, field)
			break
		}
	}
	return missing
}

func environmentKindToSQL(kinds []model.EnvironmentKind) []string {
	ret := []string{}
	for _, kind := range kinds {
		ret = append(ret, kind.String())
	}
	slices.Sort(ret)
	return ret
}

func featureFromSQL(f featuresql.FeatureDatum) (*model.Feature, error) {
	fyaml, defaultValues, err := makeFeatureYAML(f)
	if err != nil {
		return nil, fmt.Errorf("make feature yaml: %w", err)
	}

	return &model.Feature{
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

func makeFeatureYAML(fd featuresql.FeatureDatum) (model.FeatureYAML, map[string]json.RawMessage, error) {
	ret := model.FeatureYAML{
		Timeout: time.Duration(fd.Timeout) * time.Millisecond,
	}
	if err := json.Unmarshal(fd.Dependencies, &ret.Dependencies); err != nil {
		return ret, nil, fmt.Errorf("unmarshal dependencies: %w", err)
	}

	var retDefaultVals map[string]json.RawMessage
	if err := json.Unmarshal(fd.DefaultValues, &retDefaultVals); err != nil {
		return ret, nil, fmt.Errorf("unmarshal default values: %w", err)
	}

	ret.EnvironmentKinds = make([]model.EnvironmentKind, len(fd.Kinds))
	for i, k := range fd.Kinds {
		ret.EnvironmentKinds[i] = model.EnvironmentKind(k)
	}

	if err := json.Unmarshal(fd.Values, &ret.Values); err != nil {
		return ret, nil, fmt.Errorf("unmarshal values: %w", err)
	}

	return ret, retDefaultVals, nil
}
