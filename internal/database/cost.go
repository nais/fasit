package database

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
)

type CostRepo interface {
	Cost(ctx context.Context, start, end time.Time) (*model.Cost, error)
	CostForTenant(ctx context.Context, tenant uuid.UUID, start, end time.Time) (*model.TenantCosts, error)
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

func (r *repo) CostForTenant(ctx context.Context, tenant uuid.UUID, start, end time.Time) (*model.TenantCosts, error) {
	rows, err := r.querier.CostForTenant(ctx, gensql.CostForTenantParams{
		StartDate: pgtype.Date{Time: start, Valid: true},
		EndDate:   pgtype.Date{Time: end, Valid: true},
		TenantID:  tenant,
	})
	if err != nil {
		return nil, err
	}

	ret := &model.TenantCosts{
		From: start,
		To:   end,
	}

	for _, r := range rows {
		ret.Series = append(ret.Series, &model.EnvSeries{
			EnvID: r.EnvID,
			Data:  convertFloat(r.Cost),
		})
	}

	return ret, nil
}

func (r *repo) Cost(ctx context.Context, start, end time.Time) (*model.Cost, error) {
	rows, err := r.querier.Cost(ctx, gensql.CostParams{
		StartDate: pgtype.Date{Time: start, Valid: true},
		EndDate:   pgtype.Date{Time: end, Valid: true},
	})
	if err != nil {
		return nil, err
	}

	ret := &model.Cost{
		From: start,
		To:   end,
	}

	for _, r := range rows {
		ret.Series = append(ret.Series, &model.CostSeries{
			TenantID: r.TenantID,
			Data:     convertFloat(r.Cost),
		})
	}

	return ret, nil
}

func convertFloat(i []float32) []float64 {
	ret := make([]float64, len(i))
	for idx, v := range i {
		ret[idx] = float64(v)
	}
	return ret
}
