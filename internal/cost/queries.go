package cost

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/cost/costsql"
	"github.com/nais/fasit/internal/graph/model"
)

type ctxKey int

const querierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, querierKey, costsql.New(pool))
}

func querier(ctx context.Context) costsql.Querier {
	return ctx.Value(querierKey).(costsql.Querier)
}

func CostForTenant(ctx context.Context, tenant uuid.UUID, start, end time.Time) (*model.TenantCosts, error) {
	rows, err := querier(ctx).CostForTenant(ctx, costsql.CostForTenantParams{
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

func Cost(ctx context.Context, start, end time.Time) (*model.Cost, error) {
	rows, err := querier(ctx).Cost(ctx, costsql.CostParams{
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

func costLastDate(ctx context.Context) (time.Time, error) {
	d, err := querier(ctx).CostLastDate(ctx)
	if err != nil {
		return time.Time{}, err
	}

	return d.Time, nil
}

func costUpsert(ctx context.Context, rows []costsql.CostUpsertParams) error {
	res := querier(ctx).CostUpsert(ctx, rows)

	var err error
	res.Exec(func(i int, ierr error) {
		err = errors.Join(err, ierr)
	})

	return err
}

func convertFloat(i []float32) []float64 {
	ret := make([]float64, len(i))
	for idx, v := range i {
		ret[idx] = float64(v)
	}
	return ret
}
