package feature

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/errs"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
)

type Manager struct {
	querier            featuresql.Querier
	environmentManager *environment.Manager
}

func NewManager(pool *pgxpool.Pool) *Manager {
	return &Manager{
		querier:            featuresql.New(pool),
		environmentManager: environment.NewManager(pool),
	}
}

func (m *Manager) HelmValues(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
	mv, envKind, err := m.MappingValuesForEnvironment(ctx, envID, true)
	if err != nil {
		return nil, err
	}

	includeKeys := []string{}
	for key, f := range f.Values {
		if f.Config != nil && !slices.Contains(f.IgnoreKind, envKind) {
			includeKeys = append(includeKeys, key)
		}
	}

	vals, err := m.querier.ConfigForEnvironmentFilteredByKeys(ctx, featuresql.ConfigForEnvironmentFilteredByKeysParams{
		Feature:       f.Name,
		EnvironmentID: envID,
		Includedkeys:  includeKeys,
	})
	if err != nil {
		return nil, err
	}

	mp, err := makeHelmConfigMap(vals)
	if err != nil {
		return nil, err
	}

	err = Generate(f.Values, envKind, mv, mp)

	mp["fasit"] = map[string]any{
		"tenant": map[string]string{
			"name": mv.Tenant.Name,
		},
		"env": map[string]string{
			"name": mv.Env["name"].(string),
			"kind": envKind.String(),
		},
	}

	missing := validateFields(f, envKind, vals, mp)
	if len(missing) > 0 {
		return nil, &errs.ErrMissingRequiredFields{Fields: missing}
	}

	return mp, err
}

func (m *Manager) MappingValuesForEnvironment(ctx context.Context, envID uuid.UUID, showSensitive bool) (*ComputedValues, model.EnvironmentKind, error) {
	env, err := m.environmentManager.Get(ctx, envID)
	if err != nil {
		return nil, "", fmt.Errorf("envValuesForEnv: failed to get environment: %w", err)
	}

	tenant, err := m.environmentManager.GetTenant(ctx, env.TenantID)
	if err != nil {
		return nil, env.Kind, fmt.Errorf("envValuesForEnv: failed to get tenant: %w", err)
	}
	mv := &ComputedValues{
		Kind: env.Kind,
		Tenant: ComputedTenant{
			Name: tenant.Name,
		},
	}

	values, err := m.querier.ListMappingValuesForTenant(ctx, featuresql.ListMappingValuesForTenantParams{
		Tenantid:      tenant.ID,
		Showsensitive: showSensitive,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return mv, env.Kind, nil
		}
		return nil, env.Kind, fmt.Errorf("envValuesForEnv: failed to get environment values: %w", err)
	}

	for _, env := range values {
		val := map[string]any{}
		if err := json.Unmarshal(env.Values, &val); err != nil {
			return nil, model.EnvironmentKind(env.Kind), fmt.Errorf("envValuesForEnv: failed to unmarshal values for %q: %w", env.Name, err)
		}
		val["name"] = env.Name
		val["kind"] = string(env.Kind)

		if env.ID == envID {
			mv.Env = val
		}
		if env.Kind == featuresql.EnvironmentKind(model.EnvironmentKindManagement) {
			mv.Management = val
		} else {
			mv.Envs = append(mv.Envs, val)
		}
	}

	return mv, env.Kind, nil
}
