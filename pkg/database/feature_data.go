package database

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

type FeatureDataRepo interface {
	FeatureDataCreate(context.Context, model.Feature, *feature.FeatureTemplateDetails) error
}

func environmentKindToSQL(kinds []model.EnvironmentKind) []string {
	ret := []string{}
	for _, kind := range kinds {
		ret = append(ret, kind.String())
	}
	slices.Sort(ret)
	return ret
}

func (r *repo) FeatureDataCreate(ctx context.Context, feat model.Feature, details *feature.FeatureTemplateDetails) error {
	// TODO: Use pgx v5 instead of []byte
	dep, err := json.Marshal(feat.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies to json: %w", err)
	}
	vals, err := json.Marshal(feat.Values)
	if err != nil {
		return fmt.Errorf("marshal values to json: %w", err)
	}
	defaultVals, err := json.Marshal(feat.ValuesYAML)
	if err != nil {
		return fmt.Errorf("marshal default values to json: %w", err)
	}

	detailsBytes, err := json.Marshal(details)
	if err != nil {
		return fmt.Errorf("marshal details to json: %w", err)
	}

	rename, err := json.Marshal(feat.Rename)
	if err != nil {
		return fmt.Errorf("marshal rename to json: %w", err)
	}

	return r.querier.FeatureDataCreate(ctx, gensql.FeatureDataCreateParams{
		FeatureName:   feat.Name,
		Version:       feat.Version,
		Chart:         feat.Chart,
		Description:   feat.Description,
		Source:        feat.Source,
		Kinds:         environmentKindToSQL(feat.EnvironmentKinds),
		Dependencies:  dep,
		Values:        vals,
		DefaultValues: defaultVals,
		Timeout:       feat.Timeout.Milliseconds(),
		TplDetails:    detailsBytes,
		Rename:        rename,
	})
}
