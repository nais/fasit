package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
)

func (r *repo) EnvironmentValueStore(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage) error {
	return r.querier.EnvironmentValueStore(ctx, gensql.EnvironmentValueStoreParams{
		Envid: environmentID,
		Key:   key,
		Value: value,
	})
}

func (r *repo) EnvironmentValueGet(ctx context.Context, environmentID uuid.UUID, key string) (*model.EnvironmentValue, error) {
	ev, err := r.querier.EnvironmentValueGet(ctx, gensql.EnvironmentValueGetParams{
		Envid: environmentID,
		Key:   key,
	})
	if err != nil {
		return nil, err
	}

	return environmentValueFromSQL(ev), nil
}
func (r *repo) EnvironmentValuesForEnvironment(ctx context.Context, envID uuid.UUID) ([]*model.EnvironmentValue, error) {
	values, err := r.querier.EnvironmentValuesForEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}

	ret := make([]*model.EnvironmentValue, len(values))
	for i, ev := range values {
		ret[i] = environmentValueFromSQL(ev)
	}

	return ret, nil
}

func (r *repo) MappingValuesForEnvironment(ctx context.Context, envID uuid.UUID) (*feature.MappingValues, error) {
	env, err := r.querier.EnvironmentGet(ctx, envID)
	if err != nil {
		return nil, fmt.Errorf("envValuesForEnv: failed to get environment: %w", err)
	}

	tenant, err := r.TenantGet(ctx, env.TenantID)
	if err != nil {
		return nil, fmt.Errorf("envValuesForEnv: failed to get tenant: %w", err)
	}
	mv := &feature.MappingValues{
		Kind: model.EnvironmentKind(env.Kind),
		Tenant: feature.MappingTenant{
			Name: tenant.Name,
		},
	}

	evs, err := r.querier.MappingValuesForEnvironment(ctx, gensql.MappingValuesForEnvironmentParams{
		Tenantid: tenant.ID,
		Envid:    envID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mv, nil
		}
		return nil, fmt.Errorf("envValuesForEnv: failed to get environment values: %w", err)
	}

	if err := json.Unmarshal(evs.Management, &mv.Management); err != nil {
		return nil, fmt.Errorf("envValuesForEnv: failed to unmarshal management values: %w", err)
	}

	if err := json.Unmarshal(evs.Environment, &mv.Env); err != nil {
		return nil, fmt.Errorf("envValuesForEnv: failed to unmarshal environment values: %w", err)
	}

	mv.Env["name"] = env.Name

	return mv, nil
}

func environmentValueFromSQL(p gensql.EnvironmentValue) *model.EnvironmentValue {
	return &model.EnvironmentValue{
		EnvironmentID: p.EnvironmentID,
		Key:           p.Key,
		Value:         p.Value,
	}
}
