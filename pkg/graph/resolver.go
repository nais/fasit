package graph

import (
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/spanner"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	SRepo    *spanner.Repo
	Repo     *database.Repo
	Features *feature.Manager
}
