package naisdstatustest

import (
	"context"
	"testing"

	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/naisdstatus/naisdstatussql/mocks"
)

func Register(ctx context.Context, t *testing.T) context.Context {
	return context.WithValue(ctx, naisdstatus.QuerierKey, mocks.NewQuerier(t))
}

func GetQuerier(ctx context.Context) *mocks.Querier {
	return ctx.Value(naisdstatus.QuerierKey).(*mocks.Querier)
}
