package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	feature "github.com/nais/fasit/pkg/feature2"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeatureDataRepo interface {
	FeatureDataCreate(context.Context, feature.Feature) error
}

func environmentKindToSQL(kinds []model.EnvironmentKind) []string {
	ret := []string{}
	for _, kind := range kinds {
		ret = append(ret, kind.String())
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
	defaultVals, err := toJSONB(feature.ValuesYAML)
	if err != nil {
		return fmt.Errorf("marshal default values to json: %w", err)
	}

	return r.querier.FeatureDataCreate(ctx, gensql.FeatureDataCreateParams{
		FeatureName: feature.Name,
		Version:     feature.Version,
		Chart:       feature.Chart,
		Description: feature.Description,
		Source:      feature.Source,
		Kinds:       environmentKindToSQL(feature.EnvironmentKinds),
		Timeout: sql.NullInt64{
			Int64: int64(feature.Timeout / time.Microsecond),
			Valid: feature.Timeout > 0,
		},
		Dependencies:  dep,
		Values:        vals,
		DefaultValues: defaultVals,
	})
}
