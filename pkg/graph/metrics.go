package graph

import (
	"context"
	"fmt"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

type Metrics struct {
	resolverTime syncint64.Histogram
}

var _ interface {
	graphql.HandlerExtension
	graphql.ResponseInterceptor
	graphql.FieldInterceptor
} = &Metrics{}

func NewMetrics(meter metric.Meter) (*Metrics, error) {
	resTime, err := meter.Int64Histogram("gql_query_time", instrument.WithDescription("graphql gql query time"), instrument.WithUnit("ms"))
	if err != nil {
		return nil, fmt.Errorf("failed to create gql_query_time histogram: %w", err)
	}

	return &Metrics{
		resolverTime: resTime,
	}, nil
}

func (a *Metrics) ExtensionName() string {
	return "gqlgen-metrics"
}

func (a *Metrics) Validate(_ graphql.ExecutableSchema) error {
	return nil
}

func (a *Metrics) InterceptResponse(ctx context.Context, next graphql.ResponseHandler) *graphql.Response {
	return next(ctx)
}

func (a *Metrics) InterceptField(ctx context.Context, next graphql.Resolver) (interface{}, error) {
	fc := graphql.GetFieldContext(ctx)
	if !fc.IsResolver {
		return next(ctx)
	}

	start := time.Now()
	res, err := next(ctx)
	attr := attribute.Key("resolver").String(fc.Field.ObjectDefinition.Name + "/" + fc.Field.Name)
	a.resolverTime.Record(ctx, time.Since(start).Milliseconds(), attr)
	return res, err
}
