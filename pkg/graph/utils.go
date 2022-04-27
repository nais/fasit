package graph

import (
	"encoding/json"

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
