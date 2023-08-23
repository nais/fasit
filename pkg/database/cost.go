package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type CostRepo interface {
	CostByTenant(ctx context.Context, tenant uuid.UUID, start, end time.Time) ([]*model.Cost, error)
	CostLastDate(ctx context.Context) (time.Time, error)
	CostUpsert(ctx context.Context, rows []gensql.CostUpsertParams) error
}

func (r *repo) CostUpsert(ctx context.Context, rows []gensql.CostUpsertParams) error {
	res := r.querier.CostUpsert(ctx, rows)

	var err error
	res.Exec(func(i int, ierr error) {
		err = errors.Join(err, ierr)
	})

	return err
}

func (r *repo) CostLastDate(ctx context.Context) (time.Time, error) {
	d, err := r.querier.CostLastDate(ctx)
	if err != nil {
		return time.Time{}, err
	}

	return d.Time, nil
}

func (r *repo) CostByTenant(ctx context.Context, tenant uuid.UUID, start, end time.Time) ([]*model.Cost, error) {
	rows, err := r.querier.CostByTenant(ctx, gensql.CostByTenantParams{
		StartDate: pgtype.Date{Time: start, Valid: true},
		EndDate:   pgtype.Date{Time: end, Valid: true},
		TenantID:  pgtype.UUID{Bytes: tenant, Valid: tenant != uuid.Nil},
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.Cost, len(rows))
	for i, row := range rows {
		ret[i] = &model.Cost{
			TenantID: row.TenantID,
			Date:     row.Date.Time,
			Cost:     float64(row.Cost),
		}
	}

	return ret, nil
}
