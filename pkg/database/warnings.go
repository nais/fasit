package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type WarningRepo interface {
	Warnings(ctx context.Context, environmentID *uuid.UUID, tenantID *uuid.UUID) ([]model.Warning, error)
}

func (r *repo) Warnings(ctx context.Context, environmentID *uuid.UUID, tenantID *uuid.UUID) ([]model.Warning, error) {
	args := gensql.WarningsParams{}
	if environmentID == nil && tenantID == nil {
		return nil, fmt.Errorf("must specify either environmentID or tenantID")
	}
	if environmentID != nil && tenantID != nil {
		return nil, fmt.Errorf("must specify either environmentID or tenantID, not both")
	}

	if environmentID != nil {
		args.EnvironmentID = *environmentID
	}
	if tenantID != nil {
		args.TenantID = *tenantID
	}

	warnings, err := r.querier.Warnings(ctx, args)
	if err != nil {
		return nil, err
	}

	// Ensure that warnings are only returned for features that are actually in the environment
	ws := []gensql.WarningsRow{}
	for _, w := range warnings {
		if w.FeatureDataName != "" {
			ws = append(ws, w)
		} else if r.oldFeatures.Get(w.FeatureName) != nil {
			ws = append(ws, w)
		} else {
			fmt.Println("Removed warning for feature", w.FeatureName)
		}
	}

	return warningsFromSQL(ws)
}

func warningsFromSQL(warnings []gensql.WarningsRow) ([]model.Warning, error) {
	var result []model.Warning
	for _, w := range warnings {
		switch w.Type {
		case "feature_status":
			result = append(result, model.FeatureWarning{
				Message:       "feature not reconciled correctly",
				EnvironmentID: w.EnvironmentID,
				FeatureName:   w.FeatureName,
			})

		case "naisd":
			result = append(result, model.NaisdWarning{
				Message:       "naisd not healthy",
				EnvironmentID: w.EnvironmentID,
			})
		default:
			return nil, fmt.Errorf("unknown warning type: %s", w.Type)
		}
	}
	return result, nil
}
