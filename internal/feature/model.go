package feature

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
)

func makeHelmConfigMap(vals []featuresql.ConfigForEnvironmentFilteredByKeysRow) (map[string]any, error) {
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
				if p, ok := e.(map[string]any); ok {
					if index == len(keys)-1 {
						return nil, fmt.Errorf("key %v is not nestable", v.Key)
					}
					parent = p
					continue
				}
				return nil, fmt.Errorf("key %v is not nestable", v.Key)
			}
			f := make(map[string]any)
			parent[key] = f
			parent = f
		}
	}
	return val, nil
}

func validateFields(f *model.Feature, envKind model.EnvironmentKind, values []featuresql.ConfigForEnvironmentFilteredByKeysRow, mp map[string]any) []string {
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

func featuresFromSQL(rows []featuresql.FeaturesForKindRow) ([]*model.Feature, error) {
	ret := make([]*model.Feature, len(rows))
	for i, f := range rows {
		feature, err := featureFromSQL(f.FeatureDatum)
		if err != nil {
			return nil, fmt.Errorf("make feature: %w", err)
		}
		feature.HasDeployments = f.Hasdeployments
		ret[i] = feature
	}
	return ret, nil
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

	if len(fd.Rename) > 0 {
		if err := json.Unmarshal(fd.Rename, &ret.Rename); err != nil {
			return ret, nil, fmt.Errorf("unmarshal rename: %w", err)
		}
	}

	return ret, retDefaultVals, nil
}

func nullTimeToPtr(nt pgtype.Timestamptz) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func featureStateFromSQL(state featuresql.FeatureState) *model.FeatureState {
	return &model.FeatureState{
		ID:           model.FeatureStateID(state.EnvironmentID, state.Feature),
		EnvID:        state.EnvironmentID,
		FeatureName:  state.Feature,
		EnabledAt:    nullTimeToPtr(state.EnabledAt),
		Enabled:      state.Enabled,
		Created:      state.Created.Time,
		LastModified: state.LastModified.Time,
	}
}

func Now(ctx context.Context) time.Time {
	if now, ok := ctx.Value("nowfunc").(func() time.Time); ok {
		return now()
	}
	return time.Now()
}
