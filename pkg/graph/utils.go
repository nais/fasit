package graph

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/feature/helminfo"
	"github.com/nais/fasit/pkg/graph/model"
)

func marshalFeature(feat feature.Feature) (*model.Feature, error) {
	if feat.Config == nil {
		feat.Config = feature.Config{}
	}
	config, err := json.Marshal(feat.Config)
	if err != nil {
		return nil, err
	}

	deps := []*model.Dependency{}

	for _, f := range feat.DependsOn {
		deps = append(deps, &model.Dependency{
			AnyOf: f.AnyOf,
			AllOf: f.AllOf,
		})
	}

	tmp := &model.Feature{
		Name:             feat.Name,
		Chart:            feat.Chart,
		Version:          feat.Version,
		Repo:             feat.Repo,
		Source:           feat.Source,
		DependsOn:        deps,
		Config:           config,
		EnvironmentKinds: feat.EnvironmentKinds,
	}
	return tmp, nil
}

func removeIgnoredKinds(old []model.Configuration, f *feature.Feature, envKind model.EnvironmentKind) (ret []model.Configuration) {
	for key, val := range f.Config {
		for _, c := range old {
			if c.GetKey() == key {
				if val.IgnoreKind.Contains(envKind) {
					continue
				}
				ret = append(ret, c)
			}
		}
	}
	return ret
}

func contains[T comparable](s []T, value T) bool {
	for _, f := range s {
		if f == value {
			return true
		}
	}
	return false
}

func mappingToSlice(f *feature.Feature, envKind model.EnvironmentKind, env *feature.MappingValues) ([]*model.MappingValue, error) {
	target := map[string]any{}
	if err := f.Mapping.Generate(envKind, env, target); err != nil {
		return nil, err
	}

	flat := flattenMap(target)

	mapping := []*model.MappingValue{}
	for k, v := range flat {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}

		mapping = append(mapping, &model.MappingValue{
			Key:         k,
			Value:       b,
			DisplayName: f.Mapping.DisplayName(k),
		})
	}

	sort.Slice(mapping, func(i, j int) bool {
		return mapping[i].Key < mapping[j].Key
	})

	return mapping, nil
}

func flattenMap(mp map[string]any) map[string]any {
	ret := map[string]any{}
	for k, v := range mp {
		k = strings.ReplaceAll(k, ".", "\\.")
		if vMap, ok := v.(map[string]any); ok {
			mp := flattenMap(vMap)
			for k2, v2 := range mp {
				ret[k+"."+k2] = v2
			}
		} else {
			ret[k] = v
		}
	}

	return ret
}

func makeOutdatedInfo(featureName string, version *helminfo.ChartVersion) []*model.OutdatedInfo {
	if version == nil {
		return []*model.OutdatedInfo{}
	}

	if version.Outdated() {
		return []*model.OutdatedInfo{
			{
				FeatureName: featureName,
				NewVersion:  version.NewVersion,
			},
		}
	}

	ret := []*model.OutdatedInfo{}
	for _, d := range version.Dependencies {
		if d.Outdated() {
			ret = append(ret, &model.OutdatedInfo{
				FeatureName:    featureName,
				NewVersion:     d.NewVersion,
				Dependency:     true,
				DependencyName: d.Name,
			})
		}
	}

	return ret
}
