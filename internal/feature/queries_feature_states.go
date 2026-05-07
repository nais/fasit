package feature

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

// reasonDescriptionMax bounds the prefix of the disable reason embedded in
// the audit description; the full reason is in metadata.
const reasonDescriptionMax = 200

func FeatureStatesEnable(ctx context.Context, envID uuid.UUID, feature *model.Feature) (*model.FeatureState, error) {
	current, err := getFeatureStateRow(ctx, envID, feature.Name)
	if err != nil {
		return nil, err
	}
	if current != nil && current.Enabled {
		return featureStateFromSQL(*current), nil
	}

	if err := checkDependencies(ctx, envID, feature); err != nil {
		return nil, err
	}

	res, err := querier(ctx).FeatureStateCreateOrUpdate(ctx, featuresql.FeatureStateCreateOrUpdateParams{
		EnvironmentID: envID,
		Feature:       feature.Name,
		Enabled:       true,
		Enabledat:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
	})
	if err != nil {
		return nil, err
	}

	if err := audit.Create(ctx, audit.CreateParams{
		Description: "enabled",
		ObjectType:  "feature_states",
		ObjectID:    envID.String() + ":" + feature.Name,
		Metadata: map[string]any{
			"verb":    "enable",
			"feature": feature.Name,
			"envId":   envID.String(),
		},
	}); err != nil {
		return nil, err
	}

	return featureStateFromSQL(res), nil
}

func FeatureStatesDisable(ctx context.Context, envID uuid.UUID, feature *model.Feature, reason string) (*model.FeatureState, error) {
	current, err := getFeatureStateRow(ctx, envID, feature.Name)
	if err != nil {
		return nil, err
	}
	if current != nil && !current.Enabled {
		return featureStateFromSQL(*current), nil
	}

	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, fmt.Errorf("disable reason is required")
	}
	if len(reason) > 1000 {
		return nil, fmt.Errorf("disable reason must be at most 1000 chars")
	}

	res, err := querier(ctx).FeatureStateCreateOrUpdate(ctx, featuresql.FeatureStateCreateOrUpdateParams{
		EnvironmentID: envID,
		Feature:       feature.Name,
		Enabled:       false,
		Enabledat:     pgtype.Timestamptz{Valid: false},
	})
	if err != nil {
		return nil, err
	}

	descReason := reason
	if r := []rune(descReason); len(r) > reasonDescriptionMax {
		descReason = string(r[:reasonDescriptionMax])
	}

	if err := audit.Create(ctx, audit.CreateParams{
		Description: "disabled: " + descReason,
		ObjectType:  "feature_states",
		ObjectID:    envID.String() + ":" + feature.Name,
		Metadata: map[string]any{
			"verb":    "disable",
			"feature": feature.Name,
			"envId":   envID.String(),
			"reason":  reason,
		},
	}); err != nil {
		return nil, err
	}

	return featureStateFromSQL(res), nil
}

func getFeatureStateRow(ctx context.Context, envID uuid.UUID, name string) (*featuresql.FeatureState, error) {
	row, err := querier(ctx).FeatureStateGet(ctx, featuresql.FeatureStateGetParams{
		EnvironmentID: envID,
		Feature:       name,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &row, nil
}

func checkDependencies(ctx context.Context, envID uuid.UUID, feature *model.Feature) error {
	if len(feature.Dependencies) == 0 {
		return nil
	}
	states, err := FeatureStatesGet(ctx, envID)
	if err != nil {
		return err
	}
	enabledFeatures := []string{}
	for _, s := range states {
		if s.Enabled {
			enabledFeatures = append(enabledFeatures, s.FeatureName)
		}
	}
	missing := feature.Dependencies.FindMissing(enabledFeatures)
	if len(missing) > 0 {
		return fmt.Errorf("dependency '%v' not enabled", missing)
	}
	return nil
}
