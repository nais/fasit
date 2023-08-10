package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeatureDataRepo interface {
	FeatureDataCreate(context.Context, model.Feature) error
}

func environmentKindToSQL(kinds []model.EnvironmentKind) []string {
	ret := []string{}
	for _, kind := range kinds {
		ret = append(ret, kind.String())
	}
	return ret
}

func (r *repo) FeatureDataCreate(ctx context.Context, feature model.Feature) error {
	// TODO: Use pgx v5 instead of []byte
	dep, err := json.Marshal(feature.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies to json: %w", err)
	}
	vals, err := json.Marshal(feature.Values)
	if err != nil {
		return fmt.Errorf("marshal values to json: %w", err)
	}
	defaultVals, err := json.Marshal(feature.ValuesYAML)
	if err != nil {
		return fmt.Errorf("marshal default values to json: %w", err)
	}

	return r.querier.FeatureDataCreate(ctx, gensql.FeatureDataCreateParams{
		FeatureName:   feature.Name,
		Version:       feature.Version,
		Chart:         feature.Chart,
		Description:   feature.Description,
		Source:        feature.Source,
		Kinds:         environmentKindToSQL(feature.EnvironmentKinds),
		Dependencies:  dep,
		Values:        vals,
		DefaultValues: defaultVals,
		Timeout:       feature.Timeout.Milliseconds(),
	})
}
