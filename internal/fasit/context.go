package fasit

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/deployment"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/naisdstatus"
)

type SetupContextFunc func(context.Context) context.Context

func GetSetupContextFunc(pool *pgxpool.Pool, deploymentManager *deployment.Manager) SetupContextFunc {
	return func(ctx context.Context) context.Context {
		ctx = environment.Register(ctx, pool)
		ctx = feature.Register(ctx, pool)
		ctx = naisdstatus.Register(ctx, pool)
		ctx = deployment.Register(ctx, deploymentManager)
		return ctx
	}
}
