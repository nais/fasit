package database

import (
	"context"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type AutoInstallsRepo interface {
	AutoInstallsForKind(ctx context.Context, kind model.EnvironmentKind) ([]string, error)
	AutoInstallsListen(ctx context.Context, fn ListenFunc) error
}

func (r *repo) AutoInstallsForKind(ctx context.Context, kind model.EnvironmentKind) ([]string, error) {
	return r.querier.AutoInstallsForKind(ctx, gensql.EnvironmentKind(kind.String()))
}

func (r *repo) AutoInstallsListen(ctx context.Context, fn ListenFunc) error {
	return r.ListenNotify(ctx, "auto_installs_notify", fn)
}
