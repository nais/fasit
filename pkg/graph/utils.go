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
		Name:      feature.Name,
		Chart:     feature.Chart,
		Version:   feature.Version,
		Repo:      feature.Repo,
		Source:    feature.Source,
		DependsOn: feature.DependsOn,
		Config:    config,
	}
	return tmp, nil
}
