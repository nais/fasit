package uidata

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/ui/uidata/sqlgen"
)

type EnvironmentValueReferences = map[string][]string

type Environment struct {
	ID           uuid.UUID
	Name         string
	Description  *string
	Created      time.Time
	LastModified time.Time
	Kind         types.EnvironmentKind
	Reconcile    bool
	Labels       types.EnvironmentLabels
}

type EnvironmentFeature struct {
	Name            string
	FeatureDisabled bool
}

type Tenant struct {
	ID           uuid.UUID
	Name         string
	Description  *string
	Created      time.Time
	LastModified time.Time

	envs []*Environment
}

func (t *Tenant) Environments(ctx context.Context) ([]*Environment, error) {
	if len(t.envs) > 0 {
		return t.envs, nil
	}

	rows, err := querier(ctx).ListTenantEnvironments(ctx, t.ID)
	if err != nil {
		return nil, err
	}

	t.envs = make([]*Environment, len(rows))
	for i, row := range rows {
		t.envs[i] = environmentFromSQL(row)
	}

	return t.envs, nil
}

func environmentFromSQL(e sqlgen.Environment) *Environment {
	return &Environment{
		ID:           e.ID,
		Name:         e.Name,
		Description:  e.Description,
		Created:      e.Created.Time,
		LastModified: e.LastModified.Time,
		Kind:         e.Kind,
		Reconcile:    e.Reconcile,
		Labels:       e.Labels,
	}
}

func tenantFromSQL(t sqlgen.Tenant) *Tenant {
	return &Tenant{
		ID:           t.ID,
		Name:         t.Name,
		Description:  t.Description,
		Created:      t.Created.Time,
		LastModified: t.LastModified.Time,
	}
}
