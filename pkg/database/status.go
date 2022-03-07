package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/workers"
)

func (r *Repo) StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *workers.HelmStatus) error {
	return r.querier.StatusCreateOrUpdate(ctx, gensql.StatusCreateOrUpdateParams{
		Environmentid: environmentID,
		Feature:       h.Name,
		Version:       h.Version,
		Status:        h.RolloutStatus,
	})
}
