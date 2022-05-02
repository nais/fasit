package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

func (r *repo) StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Helm) error {
	return r.querier.StatusCreateOrUpdate(ctx, gensql.StatusCreateOrUpdateParams{
		EnvironmentID: environmentID,
		Feature:       h.Name,
		Version:       h.Version,
		Status:        h.RolloutStatus.String(),
		ConfigHash:    h.ConfigHash,
		Log:           h.Log,
	})
}

func (r *repo) StatusForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]*model.Status, error) {
	status, err := r.querier.StatusForEnvironment(ctx, environmentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var ret []*model.Status
	for _, s := range status {
		ret = append(ret, &model.Status{
			EnvironmentID: s.EnvironmentID,
			Feature:       s.Feature,
			Version:       s.Version,
			Status:        model.RolloutStatus(s.Status),
			ConfigHash:    s.ConfigHash,
			Created:       s.Created,
			LastModified:  s.LastModified,
			Log:           s.Log,
		})
	}

	return ret, nil
}

func (r *repo) StatusForFeature(ctx context.Context, environmentID uuid.UUID, feature string) (*model.Status, error) {
	arg := gensql.StatusForFeatureParams{
		Feature:       feature,
		EnvironmentID: environmentID,
	}
	s, err := r.querier.StatusForFeature(ctx, arg)
	if err != nil {
		return nil, err
	}
	return &model.Status{
		EnvironmentID: s.EnvironmentID,
		Feature:       s.Feature,
		Version:       s.Version,
		Status:        model.RolloutStatus(s.Status),
		ConfigHash:    s.ConfigHash,
		Created:       s.Created,
		LastModified:  s.LastModified,
		Log:           s.Log,
	}, nil
}
