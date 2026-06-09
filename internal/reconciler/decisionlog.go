package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

// DecisionLogEntry is a single reconciler decision recorded for a feature in an
// environment. A row exists only for cycles where the decision changed, so the
// list reads as a sparse transition timeline.
type DecisionLogEntry struct {
	ID                  int64
	FeatureAssignmentID uuid.UUID
	FeatureVersion      string
	Action              string
	Message             string
	Created             time.Time
}

// ListDecisionLog returns the decision history for a feature in an environment,
// newest first.
func ListDecisionLog(ctx context.Context, envID uuid.UUID, featureName string) ([]*DecisionLogEntry, error) {
	rows, err := querier(ctx).ListDecisionLog(ctx, reconcilersql.ListDecisionLogParams{
		EnvironmentID: envID,
		FeatureName:   featureName,
	})
	if err != nil {
		return nil, fmt.Errorf("list decision log: %w", err)
	}
	entries := make([]*DecisionLogEntry, len(rows))
	for i, r := range rows {
		entries[i] = &DecisionLogEntry{
			ID:                  r.ID,
			FeatureAssignmentID: r.FeatureAssignmentID,
			FeatureVersion:      r.FeatureVersion,
			Action:              r.Action,
			Message:             r.Message,
			Created:             r.Created,
		}
	}
	return entries, nil
}
