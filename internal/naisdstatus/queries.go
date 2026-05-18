package naisdstatus

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatussql"
)

type ctxKey int

// QuerierKey is exposed so tests can inject fake queriers on the context.
const QuerierKey ctxKey = iota

func Register(ctx context.Context, pool *pgxpool.Pool) context.Context {
	return context.WithValue(ctx, QuerierKey, naisdstatussql.New(pool))
}

func querier(ctx context.Context) naisdstatussql.Querier {
	return ctx.Value(QuerierKey).(naisdstatussql.Querier)
}

func Get(ctx context.Context, environmentID uuid.UUID) (*model.Health, error) {
	res, err := querier(ctx).Get(ctx, environmentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &model.Health{
				ReportedAt: time.Date(1969, 6, 9, 6, 9, 6, 9, time.UTC),
			}, nil
		}
		return nil, err
	}
	return &model.Health{
		EnvironmentID: res.EnvironmentID,
		ReportedAt:    res.ReportedAt.Time,
	}, nil
}

func Set(ctx context.Context, environmentID uuid.UUID, h *message.Health) error {
	_, err := querier(ctx).Set(ctx, naisdstatussql.SetParams{
		EnvironmentID: environmentID,
		ReportedAt: pgtype.Timestamptz{
			Time:  h.ReportedAt,
			Valid: true,
		},
	})

	return err
}
