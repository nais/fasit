package database

import (
	"context"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
)

type DefaultFeaturesRepo interface {
	DefaultFeaturesForKind(ctx context.Context, kind model.EnvironmentKind) ([]string, error)
}

func (r *repo) DefaultFeaturesForKind(ctx context.Context, kind model.EnvironmentKind) ([]string, error) {
	return r.querier.DefaultFeaturesForKind(ctx, gensql.EnvironmentKind(kind.String()))
}
