package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nais/fasit/pkg/graph/model"
)

func (r *Repo) ReconcileData(ctx context.Context) ([]*model.ReconcileData, error) {
	data, err := r.querier.ReconcileData(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	var ret []*model.ReconcileData
	for _, d := range data {
		ret = append(ret, &model.ReconcileData{
			Environment: model.Environment{
				ID:           model.ID(d.ID),
				Name:         d.Name,
				Description:  nullStringToPtr(d.Description),
				Created:      d.Created,
				LastModified: d.LastModified,
			},
			PartnerName: d.PartnerName,
		})
	}

	return ret, nil
}
