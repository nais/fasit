package database

import (
	"context"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type AutoInstallsRepo interface {
	AutoInstallsForKind(ctx context.Context, kind model.EnvironmentKind) ([]string, error)
}

func (r *repo) AutoInstallsForKind(ctx context.Context, kind model.EnvironmentKind) ([]string, error) {
	return r.querier.AutoInstallNamesForKind(ctx, gensql.EnvironmentKind(kind.String()))
}
