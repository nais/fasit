package feature

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/feature/featuresql"
)

// reasonDescriptionMax bounds the prefix of the disable reason embedded in
// the audit description; the full reason is in metadata.
const reasonDescriptionMax = 200

// FeatureDisable marks a (feature, environment) combination as disabled in the
// authoritative disabled_features table and writes an audit entry. Insert and
// audit are committed together via dbtx.WithTx; nested calls reuse the outer
// transaction.
func FeatureDisable(ctx context.Context, envID uuid.UUID, featureName, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return fmt.Errorf("disable reason is required")
	}
	if len(reason) > 1000 {
		return fmt.Errorf("disable reason must be at most 1000 chars")
	}

	return dbtx.WithTx(ctx, func(ctx context.Context) error {
		if err := querier(ctx).DisabledFeatureSet(ctx, featuresql.DisabledFeatureSetParams{
			EnvironmentID: envID,
			Feature:       featureName,
		}); err != nil {
			return err
		}

		// Audit description is bounded so list rendering stays cheap; the
		// full reason is preserved in metadata for callers that need it.
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
	})
}

// FeatureEnable removes a (feature, environment) row from disabled_features
// and writes an audit entry. No-op if the row does not exist. Delete and
// audit are committed together via dbtx.WithTx.
func FeatureEnable(ctx context.Context, envID uuid.UUID, featureName string) error {
	return dbtx.WithTx(ctx, func(ctx context.Context) error {
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
	})
}

// FeatureDisabledAt reports whether a (feature, environment) row exists in
// disabled_features. If disabled, returns the disabled_at timestamp and
// true; otherwise the zero time and false.
func FeatureDisabledAt(ctx context.Context, envID uuid.UUID, featureName string) (time.Time, bool, error) {
	row, err := querier(ctx).DisabledFeatureGet(ctx, featuresql.DisabledFeatureGetParams{
		EnvironmentID: envID,
		Feature:       featureName,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, err
	}
	return row.DisabledAt.Time, true, nil
}

// FeaturesDisabledIn returns the set of feature names disabled in the given
// environment. Useful for batch lookups when rendering lists.
func FeaturesDisabledIn(ctx context.Context, envID uuid.UUID) (map[string]struct{}, error) {
	rows, err := querier(ctx).DisabledFeaturesByEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, r := range rows {
		out[r.Feature] = struct{}{}
	}
	return out, nil
}
