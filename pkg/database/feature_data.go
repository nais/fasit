package database

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeatureData interface {
	FeatureDataCreate(context.Context, feature.Feature) error
}

func environmentKindToSQL(kinds []model.EnvironmentKind) []gensql.EnvironmentKind {
	ret := []gensql.EnvironmentKind{}
	for _, kind := range kinds {
		ret = append(ret, gensql.EnvironmentKind(kind))
	}
	return ret
}

func toJSONB(v any) (pgtype.JSONB, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return pgtype.JSONB{}, err
	}
	return pgtype.JSONB{Bytes: b, Status: pgtype.Present}, nil
}

func (r *repo) FeatureDataCreate(ctx context.Context, feature feature.Feature) error {
	dep, err := toJSONB(feature.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies to json: %w", err)
	}
	vals, err := toJSONB(feature.Values)
	if err != nil {
		return fmt.Errorf("marshal values to json: %w", err)
	}
	defaultVals, err := toJSONB(feature.ValuesYaml)
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
	})
}
