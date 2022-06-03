package graph

import (
	"encoding/json"
	"sort"

	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

func marshalFeature(feature feature.Feature) (*model.Feature, error) {
	config, err := json.Marshal(feature.Config)
	if err != nil {
		return nil, err
	}
	tmp := &model.Feature{
		Name:             feature.Name,
		Chart:            feature.Chart,
		Version:          feature.Version,
		Repo:             feature.Repo,
		Source:           feature.Source,
		DependsOn:        feature.DependsOn,
		Config:           config,
		EnvironmentKinds: feature.EnvironmentKinds,
	}
	return tmp, nil
}

func contains[T comparable](s []T, value T) bool {
	for _, f := range s {
		if f == value {
			return true
		}
	}
	return false
}

func mappingToSlice(f *feature.Feature, env *feature.MappingValues) ([]*model.MappingValue, error) {
	target := map[string]any{}
	if err := f.Mapping.Generate(env, target); err != nil {
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
