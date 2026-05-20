package database

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type queryCtxKey struct{}

type queryCtxVal struct {
	name  string
	start time.Time
}

var (
	sqlcNameRe = regexp.MustCompile(`^-- name: (\w+)`)
	tableRe    = regexp.MustCompile(`(?i)(?:FROM|INTO|UPDATE|JOIN)\s+(\w+)`)
)

func queryName(sql string) string {
	name := "unknown"
	if m := sqlcNameRe.FindStringSubmatch(sql); m != nil {
		name = m[1]
	}

	if m := tableRe.FindStringSubmatch(sql); m != nil {
		return strings.ToLower(m[1]) + "." + name
	}

	return name
}

type QueryMetricsTracer struct {
	queryCount    metric.Int64Counter
	queryDuration metric.Float64Histogram
}

func NewQueryMetricsTracer(meter metric.Meter) (*QueryMetricsTracer, error) {
	queryCount, err := meter.Int64Counter("db_queries_total",
		metric.WithDescription("Total database queries executed"),
	)
	if err != nil {
		return nil, err
	}

	queryDuration, err := meter.Float64Histogram("db_query_duration_seconds",
		metric.WithDescription("Database query duration"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	return &QueryMetricsTracer{
		queryCount:    queryCount,
		queryDuration: queryDuration,
	}, nil
}

func (t *QueryMetricsTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, queryCtxKey{}, queryCtxVal{
		name:  queryName(data.SQL),
		start: time.Now(),
	})
}

func (t *QueryMetricsTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	v, ok := ctx.Value(queryCtxKey{}).(queryCtxVal)
	if !ok {
		return
	}

	attrs := metric.WithAttributes(
		attribute.String("query", v.name),
		attribute.Bool("error", data.Err != nil),
	)

	t.queryCount.Add(ctx, 1, attrs)
	t.queryDuration.Record(ctx, time.Since(v.start).Seconds(), attrs)
}

func RegisterPoolMetrics(meter metric.Meter, pool *pgxpool.Pool) error {
	if _, err := meter.Int64ObservableGauge("db_pool_total_conns",
		metric.WithDescription("Total number of connections in the pool"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(pool.Stat().TotalConns()))
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := meter.Int64ObservableGauge("db_pool_idle_conns",
		metric.WithDescription("Number of idle connections in the pool"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(pool.Stat().IdleConns()))
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := meter.Int64ObservableGauge("db_pool_acquired_conns",
		metric.WithDescription("Number of acquired connections in the pool"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(pool.Stat().AcquiredConns()))
			return nil
		}),
	); err != nil {
		return err
	}

	if _, err := meter.Int64ObservableGauge("db_pool_max_conns",
		metric.WithDescription("Maximum number of connections in the pool"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(int64(pool.Stat().MaxConns()))
			return nil
		}),
	); err != nil {
		return err
	}

	return nil
}
