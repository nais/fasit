package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/message"
)

func (r *repo) ReleaseStatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Release) error {
	_, err := r.querier.ReleaseStatusCreateOrUpdate(ctx, gensql.ReleaseStatusCreateOrUpdateParams{
		EnvironmentID: environmentID,
		Feature:       h.Name,
		Version:       h.Version,
		Status:        h.Status,
		Revision:      int32(h.Revision),
		LastDeployed:  h.LastDeployed,
	})

	return err
}
