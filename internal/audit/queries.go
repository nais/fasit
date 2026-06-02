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
		Feature:       p.Feature,
		EnvironmentID: p.EnvironmentID,
		Metadata:      meta,
	})
}

func ListForFeature(ctx context.Context, feature string, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListForFeature(ctx, auditsql.ListForFeatureParams{
		Feature:  feature,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func ListAssignmentsForFeature(ctx context.Context, feature string, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListAssignmentsForFeature(ctx, auditsql.ListAssignmentsForFeatureParams{
		Feature:  feature,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func ListForFeatureInEnvironment(ctx context.Context, feature string, envID uuid.UUID, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListForFeatureInEnvironment(ctx, auditsql.ListForFeatureInEnvironmentParams{
		Feature:  feature,
		EnvID:    &envID,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func ListConfigForFeatureInEnvironment(ctx context.Context, feature string, envID uuid.UUID, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListConfigForFeatureInEnvironment(ctx, auditsql.ListConfigForFeatureInEnvironmentParams{
		Feature:  feature,
		EnvID:    &envID,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func ListGlobalConfigForFeature(ctx context.Context, feature string, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListGlobalConfigForFeature(ctx, auditsql.ListGlobalConfigForFeatureParams{
		Feature:  feature,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func ListRecent(ctx context.Context, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).ListRecent(ctx, limit)
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func SearchRecent(ctx context.Context, terms []string, limit int32) ([]*Entry, error) {
	rows, err := querier(ctx).SearchRecent(ctx, auditsql.SearchRecentParams{
		Terms:    terms,
		PageSize: limit,
	})
	if err != nil {
		return nil, err
	}
	ret := make([]*Entry, 0, len(rows))
	for _, r := range rows {
		ret = append(ret, &Entry{
			Actor: r.Actor, Action: Action(r.Action), Description: r.Description,
			ObjectType: ObjectType(r.ObjectType), ObjectID: r.ObjectID,
			EnvironmentID: r.EnvironmentID, EnvironmentName: ptrOr(r.EnvironmentName),
			TenantName: ptrOr(r.TenantName), CreatedAt: r.CreatedAt.Time, Metadata: r.Metadata,
		})
	}
	return ret, nil
}

func ptrOr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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

func LatestDisableReason(ctx context.Context, feature string, envID uuid.UUID) string {
	desc, err := querier(ctx).LatestDisableReason(ctx, auditsql.LatestDisableReasonParams{
		Feature: feature,
		EnvID:   &envID,
	})
	if err != nil {
		return ""
	}
	return desc
}
