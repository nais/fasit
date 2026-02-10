package environmenttest

import (
	"context"
	"testing"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/environment/environmentsql/mocks"
)

// RegisterMock allows overriding the database querier in tests by registering a mock querier in the context.
// Avoid usage by e.g. using testcontainers.
func RegisterMock(ctx context.Context, t *testing.T) context.Context {
	return context.WithValue(ctx, environment.QuerierKey, mocks.NewQuerier(t))
}

func GetQuerier(ctx context.Context) *mocks.Querier {
	return ctx.Value(environment.QuerierKey).(*mocks.Querier)
}
