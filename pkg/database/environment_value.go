package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	feature "github.com/nais/fasit/pkg/feature2"
	"github.com/nais/fasit/pkg/graph/model"
)

type EnvironmentValueRepo interface {
	EnvironmentValueGet(ctx context.Context, environmentID uuid.UUID, key string, showSensitive bool) (*model.EnvironmentValue, error)
	EnvironmentValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) ([]*model.EnvironmentValue, error)
	EnvironmentValueStore(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage, secret bool) error
	MappingValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) (*feature.ComputedValues, model.EnvironmentKind, error)
}

func (r *repo) EnvironmentValueStore(ctx context.Context, environmentID uuid.UUID, key string, value json.RawMessage, secret bool) error {
	err := r.querier.EnvironmentValueStore(ctx, gensql.EnvironmentValueStoreParams{
		Envid: environmentID,
		Key:   key,
		Value: pgtype.JSONB{
			Bytes:  value,
			Status: pgtype.Present,
		},
		Secret: secret,
	})
	if err != nil {
		return fmt.Errorf("failed to store environment value: %w", err)
	}

	r.createAudit(ctx, "created or updated", "environment_values", environmentID.String()+":"+key)

	return nil
}

func (r *repo) EnvironmentValueGet(ctx context.Context, environmentID uuid.UUID, key string, showSensitive bool) (*model.EnvironmentValue, error) {
	ev, err := r.querier.EnvironmentValueGet(ctx, gensql.EnvironmentValueGetParams{
		Envid:         environmentID,
		Key:           key,
		Showsensitive: showSensitive,
	})
	if err != nil {
		return nil, err
	}

	return &model.EnvironmentValue{
		EnvironmentID: ev.EnvironmentID,
		Key:           ev.Key,
		Value:         ev.Value.Bytes,
		Secret:        ev.Secret,
	}, nil
}

func (r *repo) EnvironmentValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) ([]*model.EnvironmentValue, error) {
	values, err := r.querier.EnvironmentValuesForEnvironment(ctx, gensql.EnvironmentValuesForEnvironmentParams{
		Envid:         envID,
		Showsensitive: showSensitive,
	})
	if err != nil {
		return nil, err
	}

	ret := make([]*model.EnvironmentValue, len(values))
	for i, ev := range values {
		ret[i] = &model.EnvironmentValue{
			EnvironmentID: ev.EnvironmentID,
			Key:           ev.Key,
			Value:         ev.Value.Bytes,
			Secret:        ev.Secret,
		}
	}

	return ret, nil
}

func (r *repo) MappingValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) (*feature.ComputedValues, model.EnvironmentKind, error) {
	env, err := r.querier.EnvironmentGet(ctx, envID)
	if err != nil {
		return nil, "", fmt.Errorf("envValuesForEnv: failed to get environment: %w", err)
	}

	tenant, err := r.TenantGet(ctx, env.TenantID)
	if err != nil {
		return nil, model.EnvironmentKind(env.Kind), fmt.Errorf("envValuesForEnv: failed to get tenant: %w", err)
	}
	mv := &feature.ComputedValues{
		Kind: model.EnvironmentKind(env.Kind),
		Tenant: feature.ComputedTenant{
			Name: tenant.Name,
		},
	}

	values, err := r.querier.MappingValuesForTenant(ctx, gensql.MappingValuesForTenantParams{
		Tenantid:      tenant.ID,
		Showsensitive: showSensitive,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mv, model.EnvironmentKind(env.Kind), nil
		}
		return nil, model.EnvironmentKind(env.Kind), fmt.Errorf("envValuesForEnv: failed to get environment values: %w", err)
	}

	for _, env := range values {
		val := map[string]any{}
		if err := json.Unmarshal(env.Values.Bytes, &val); err != nil {
			return nil, model.EnvironmentKind(env.Kind), fmt.Errorf("envValuesForEnv: failed to unmarshal values for %q: %w", env.Name, err)
		}
		val["name"] = env.Name
		val["kind"] = string(env.Kind)

		if env.ID == envID {
			mv.Env = val
		}
		if env.Kind == gensql.EnvironmentKind(model.EnvironmentKindManagement) {
			mv.Management = val
		} else {
			mv.Envs = append(mv.Envs, val)
		}
	}

	return mv, model.EnvironmentKind(env.Kind), nil
}
