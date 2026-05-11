package feature

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/feature/featuresql"
)

// FeatureDisable marks a (feature, environment) combination as disabled in the
// authoritative disabled_features table and writes an audit entry.
//
// This is the new-GUI write path. Old GraphQL mutations still write to
// feature_states; rollouts still read feature_states. See sesjon
// fasit-explicit-feature-disabled for context.
func FeatureDisable(ctx context.Context, envID uuid.UUID, featureName, reason string) error {
	if err := querier(ctx).DisabledFeatureSet(ctx, featuresql.DisabledFeatureSetParams{
		EnvironmentID: envID,
		Feature:       featureName,
	}); err != nil {
		return err
	}

	descReason := reason
	if r := []rune(descReason); len(r) > reasonDescriptionMax {
		descReason = string(r[:reasonDescriptionMax])
	}

	return audit.Create(ctx, audit.CreateParams{
		Description: "disabled: " + descReason,
		ObjectType:  "disabled_features",
		ObjectID:    envID.String() + ":" + featureName,
		Metadata: map[string]any{
			"verb":    "disable",
			"feature": featureName,
			"envId":   envID.String(),
			"reason":  reason,
		},
	})
}

// FeatureEnable removes a (feature, environment) row from disabled_features
// and writes an audit entry. No-op if the row does not exist.
func FeatureEnable(ctx context.Context, envID uuid.UUID, featureName string) error {
	if err := querier(ctx).DisabledFeatureDelete(ctx, featuresql.DisabledFeatureDeleteParams{
		EnvironmentID: envID,
		Feature:       featureName,
	}); err != nil {
		return err
	}

	return audit.Create(ctx, audit.CreateParams{
		Description: "enabled",
		ObjectType:  "disabled_features",
		ObjectID:    envID.String() + ":" + featureName,
		Metadata: map[string]any{
			"verb":    "enable",
			"feature": featureName,
			"envId":   envID.String(),
		},
	})
}

// FeatureDisabledAt reports whether a (feature, environment) row exists in
// disabled_features. Returns a non-nil timestamp (disabled_at) if disabled,
// nil if enabled.
func FeatureDisabledAt(ctx context.Context, envID uuid.UUID, featureName string) (*time.Time, error) {
	row, err := querier(ctx).DisabledFeatureGet(ctx, featuresql.DisabledFeatureGetParams{
		EnvironmentID: envID,
		Feature:       featureName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	at := row.DisabledAt.Time
	return &at, nil
}

// FeaturesDisabledIn returns the set of feature names disabled in the given
// environment. Useful for batch lookups when rendering lists.
func FeaturesDisabledIn(ctx context.Context, envID uuid.UUID) (map[string]bool, error) {
	rows, err := querier(ctx).DisabledFeaturesByEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(rows))
	for _, r := range rows {
		out[r.Feature] = true
	}
	return out, nil
}
