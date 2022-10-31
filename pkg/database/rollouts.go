package database

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type RolloutRepo interface {
	RolloutCreate(ctx context.Context, rollout *model.Rollout) (*model.Rollout, error)
	RolloutGetByID(ctx context.Context, id uuid.UUID) (*model.Rollout, error)
	RolloutsListen(ctx context.Context, fn ListenFunc) error
	RolloutsUnprocessed(ctx context.Context) ([]*model.Rollout, error)
	RolloutUpdate(ctx context.Context, rollout *model.Rollout) error
	RolloutsUpdateStatus(ctx context.Context, ids []uuid.UUID, status model.RolloutStatus) error

	RolloutEventCreate(ctx context.Context, event *model.RolloutEvent) error
	RolloutEventsGetByRolloutID(ctx context.Context, rolloutID uuid.UUID) ([]*model.RolloutEvent, error)
	RolloutEventsGetByRolloutIDAndType(ctx context.Context, rolloutID uuid.UUID, eventType model.RolloutEventType) ([]*model.RolloutEvent, error)

	RolloutSummaryCreate(ctx context.Context, feature string) (uuid.UUID, error)
	RolloutsBySummaryID(ctx context.Context, summaryID uuid.UUID) ([]*model.Rollout, error)
	RolloutSummaryDone(ctx context.Context, rolloutID uuid.UUID) (bool, error)
	RolloutSummaryGetByID(ctx context.Context, id uuid.UUID) (*model.RolloutSummary, error)
}

var _ RolloutRepo = &repo{}

func (r *repo) RolloutCreate(ctx context.Context, rollout *model.Rollout) (*model.Rollout, error) {
	rm, err := json.Marshal(rollout.Changeset)
	if err != nil {
		return nil, err
	}

	roll, err := r.querier.RolloutCreate(ctx, gensql.RolloutCreateParams{
		Feature: rollout.Feature,
		Changeset: pgtype.JSONB{
			Bytes:  rm,
			Status: pgtype.Present,
		},
		Rolloutsummaryid: rollout.RolloutSummaryID,
		Envkind:          gensql.EnvironmentKind(rollout.EnvironmentKind),
	})
	if err != nil {
		return nil, err
	}

	return rolloutFromSQL(roll)
}

func (r *repo) RolloutGetByID(ctx context.Context, id uuid.UUID) (*model.Rollout, error) {
	roll, err := r.querier.RolloutGetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return rolloutFromSQL(roll)
}

func (r *repo) RolloutsListen(ctx context.Context, fn ListenFunc) error {
	return r.ListenNotify(ctx, "rollout_notify", fn)
}

func (r *repo) RolloutUpdate(ctx context.Context, rollout *model.Rollout) error {
	rm, err := json.Marshal(rollout.Changeset)
	if err != nil {
		return err
	}

	nw, err := r.querier.RolloutUpdate(ctx, gensql.RolloutUpdateParams{
		ID: rollout.ID,
		Changeset: pgtype.JSONB{
			Bytes:  rm,
			Status: pgtype.Present,
		},
		Status: gensql.RolloutStatus(rollout.Status),
	})
	if err != nil {
		return err
	}

	rollout.LastModified = nw.LastModified
	return nil
}

func (r *repo) RolloutsUnprocessed(ctx context.Context) ([]*model.Rollout, error) {
	rollouts, err := r.querier.RolloutsUnprocessed(ctx)
	if err != nil {
		return nil, err
	}

	var result []*model.Rollout
	for _, roll := range rollouts {
		rollout, err := rolloutFromSQL(roll)
		if err != nil {
			return nil, err
		}

		result = append(result, rollout)
	}

	return result, nil
}

func (r *repo) RolloutsUpdateStatus(ctx context.Context, ids []uuid.UUID, status model.RolloutStatus) error {
	return r.querier.RolloutsUpdateStatus(ctx, gensql.RolloutsUpdateStatusParams{
		Ids:    ids,
		Status: gensql.RolloutStatus(status),
	})
}

func rolloutFromSQL(roll gensql.Rollout) (*model.Rollout, error) {
	cs := &model.RolloutChangeset{}

	if err := json.Unmarshal(roll.Changeset.Bytes, cs); err != nil {
		return nil, err
	}

	return &model.Rollout{
		ID:               roll.ID,
		RolloutSummaryID: roll.RolloutSummaryID,
		Feature:          roll.Feature,
		EnvironmentKind:  model.EnvironmentKind(roll.Kind),
		Status:           model.RolloutStatus(roll.Status),
		Changeset:        cs,
		Created:          roll.Created,
		LastModified:     roll.LastModified,
	}, nil
}

func (r *repo) RolloutEventCreate(ctx context.Context, event *model.RolloutEvent) error {
	jsonb := event.Data
	if jsonb == nil {
		jsonb = []byte("{}")
	}
	return r.querier.RolloutEventCreate(ctx, gensql.RolloutEventCreateParams{
		RolloutID: event.RolloutID,
		Type:      string(event.Type),
		Data: pgtype.JSONB{
			Bytes:  jsonb,
			Status: pgtype.Present,
		},
	})
}

func (r *repo) RolloutEventsGetByRolloutID(ctx context.Context, rolloutID uuid.UUID) ([]*model.RolloutEvent, error) {
	events, err := r.querier.RolloutEventsGetByRolloutID(ctx, rolloutID)
	if err != nil {
		return nil, err
	}

	var result []*model.RolloutEvent
	for _, event := range events {
		result = append(result, &model.RolloutEvent{
			ID:        event.ID,
			RolloutID: event.RolloutID,
			Type:      model.RolloutEventType(event.Type),
			Data:      event.Data.Bytes,
			Created:   event.Created,
		})
	}

	return result, nil
}

func (r *repo) RolloutEventsGetByRolloutIDAndType(ctx context.Context, rolloutID uuid.UUID, eventType model.RolloutEventType) ([]*model.RolloutEvent, error) {
	events, err := r.querier.RolloutEventsGetByRolloutIDAndType(ctx, gensql.RolloutEventsGetByRolloutIDAndTypeParams{
		RolloutID: rolloutID,
		Type:      string(eventType),
	})
	if err != nil {
		return nil, err
	}

	var result []*model.RolloutEvent
	for _, event := range events {
		result = append(result, &model.RolloutEvent{
			ID:        event.ID,
			RolloutID: event.RolloutID,
			Type:      model.RolloutEventType(event.Type),
			Data:      event.Data.Bytes,
			Created:   event.Created,
		})
	}

	return result, nil
}

func (r *repo) RolloutSummaryCreate(ctx context.Context, feature string) (uuid.UUID, error) {
	summary, err := r.querier.RolloutSummaryCreate(ctx, feature)
	if err != nil {
		return uuid.Nil, err
	}

	return summary.ID, nil
}

func (r *repo) RolloutsBySummaryID(ctx context.Context, summaryID uuid.UUID) ([]*model.Rollout, error) {
	rollouts, err := r.querier.RolloutsBySummaryID(ctx, summaryID)
	if err != nil {
		return nil, err
	}

	var result []*model.Rollout
	for _, roll := range rollouts {
		rollout, err := rolloutFromSQL(roll)
		if err != nil {
			return nil, err
		}

		result = append(result, rollout)
	}

	return result, nil
}

func (r *repo) RolloutSummaryDone(ctx context.Context, rolloutID uuid.UUID) (bool, error) {
	return r.querier.RolloutSummaryDone(ctx, rolloutID)
}

func (r *repo) RolloutSummaryGetByID(ctx context.Context, id uuid.UUID) (*model.RolloutSummary, error) {
	summary, err := r.querier.RolloutSummaryGetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return &model.RolloutSummary{
		ID:           summary.ID,
		FeatureName:  summary.Feature,
		Status:       model.RolloutStatus(summary.Status),
		Created:      summary.Created,
		LastModified: summary.LastModified,
	}, nil
}
