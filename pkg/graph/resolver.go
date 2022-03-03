package graph

import "github.com/nais/c3po/pkg/database"

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Repo *database.Repo
}
