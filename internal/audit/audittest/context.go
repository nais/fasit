// Package audittest provides test helpers to inject a mock audit querier on
// the context.
package audittest

import (
	"context"
	"testing"

	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsql/mocks"
	"github.com/sirupsen/logrus/hooks/test"
)

func RegisterMock(ctx context.Context, t *testing.T) context.Context {
	log, _ := test.NewNullLogger()
	return audit.RegisterTestDeps(ctx, mocks.NewQuerier(t), log)
}

func GetQuerier(ctx context.Context) *mocks.Querier {
	return ctx.Value(audit.QuerierKey).(*mocks.Querier)
}
