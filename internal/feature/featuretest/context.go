package featuretest

import (
	"context"
	"testing"

	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featuresql/mocks"
)

// RegisterMock allows overriding the database querier in tests by registering a mock querier in the context.
// Avoid usage by e.g. using testcontainers.
func RegisterMock(ctx context.Context, t *testing.T) context.Context {
	return context.WithValue(ctx, feature.QuerierKey, mocks.NewQuerier(t))
}

func GetQuerier(ctx context.Context) *mocks.Querier {
	return ctx.Value(feature.QuerierKey).(*mocks.Querier)
}
