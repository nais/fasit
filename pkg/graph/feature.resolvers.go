package graph

// This file will be automatically regenerated based on the schema, any resolver implementations
// will be copied through when generating and any unknown code will be moved to the end.

import (
	"context"
	"encoding/json"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *queryResolver) FeaturesGet(ctx context.Context) ([]*model.Feature, error) {
	features := []*model.Feature{}
	for _, feature := range r.Features.Features {
		config, err := json.Marshal(feature.Config)
		if err != nil {
			return nil, err
		}
		tmp := &model.Feature{
			Name:    feature.Name,
			Chart:   &feature.Chart,
			Version: feature.Version,
			Repo:    feature.Repo,
			Source:  feature.Source,
			Config:  config,
		}
		features = append(features, tmp)
	}
	return features, nil
}
