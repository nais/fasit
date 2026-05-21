package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/auth"
	"github.com/sirupsen/logrus"
)

const unknownActor = "unknown"

func Create(ctx context.Context, p CreateParams) error {
	actor := actorOrUnknown(ctx, p.Description, string(p.ObjectType))
	var meta []byte
	if p.Metadata != nil {
		b, err := json.Marshal(p.Metadata)
		if err != nil {
			return fmt.Errorf("marshal audit metadata: %w", err)
		}
		meta = b
	}

	return querier(ctx).Create(ctx, auditsql.CreateParams{
		Actor:         actor,
		Action:        string(p.Action),
		Description:   p.Description,
		ObjectType:    string(p.ObjectType),
		ObjectID:      p.ObjectID,
		EnvironmentID: p.EnvironmentID,
		Metadata:      meta,
	})
}

func List(ctx context.Context, environmentId uuid.UUID, featureName string) ([]*Entry, error) {
	rows, err := querier(ctx).List(ctx, auditsql.ListParams{
		EnvironmentID: environmentId.String(),
		FeatureName:   featureName,
		PageSize:      50,
	})
	if err != nil {
		return nil, err
	}

	return entriesFromRows(rows), nil
}

func ListForEnvironment(ctx context.Context, envID uuid.UUID, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListForEnvironment(ctx, auditsql.ListForEnvironmentParams{
		EnvID:    &envID,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}

	return entriesFromRows(rows), nil
}

func ListRecent(ctx context.Context, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListRecent(ctx, limit)
	if err != nil {
		return nil, err
	}

	return entriesFromRows(rows), nil
}

func entriesFromRows(rows []auditsql.Audit) []*Entry {
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor:         r.Actor,
			Action:        Action(r.Action),
			Description:   r.Description,
			ObjectType:    ObjectType(r.ObjectType),
			ObjectID:      r.ObjectID,
			EnvironmentID: r.EnvironmentID,
			CreatedAt:     r.CreatedAt.Time,
			Metadata:      r.Metadata,
		})
	}
	return ret
}

func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func actorOrUnknown(ctx context.Context, description, objectType string) string {
	actor := auth.GetEmail(ctx)
	if actor == auth.UnauthorizedName {
		log(ctx).WithFields(logrus.Fields{
			"description": sanitizeForLog(description),
			"objectType":  sanitizeForLog(objectType),
		}).Warn("unknown actor")
		return unknownActor
	}
	return actor
}
