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

		return audit.Create(ctx, audit.CreateParams{
			Action:        audit.ActionDisabled,
			Description:   reason,
			ObjectType:    audit.ObjectTypeFeature,
			ObjectID:      featureName,
			Feature:       featureName,
			EnvironmentID: &envID,
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
			Action:        audit.ActionEnabled,
			ObjectType:    audit.ObjectTypeFeature,
			ObjectID:      featureName,
			Feature:       featureName,
			EnvironmentID: &envID,
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
	return row.DisabledAt, true, nil
}
