package deploymenttest

import (
	"context"

	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/deployment/deploymentsql"
)

func RegisterWithQuerier(ctx context.Context, querier deploymentsql.Querier) context.Context {
	return deployment.RegisterForTest(ctx, querier)
}
