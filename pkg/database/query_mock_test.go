package database

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
)

type MockQuerier struct {
	configDelete               func(ctx context.Context, id uuid.UUID) error
	configForEnv               func(ctx context.Context, arg gensql.ConfigForEnvParams) ([]gensql.ConfigForEnvRow, error)
	configGet                  func(ctx context.Context, feature string) ([]gensql.Configuration, error)
	configGetForEnv            func(ctx context.Context, arg gensql.ConfigGetForEnvParams) ([]gensql.Configuration, error)
	configUpdate               func(ctx context.Context, arg gensql.ConfigUpdateParams) (gensql.Configuration, error)
	configUpdateOrCreate       func(ctx context.Context, arg gensql.ConfigUpdateOrCreateParams) (gensql.Configuration, error)
	envConfig                  func(ctx context.Context, arg gensql.EnvConfigParams) ([]gensql.EnvConfigRow, error)
	environmentCreate          func(ctx context.Context, arg gensql.EnvironmentCreateParams) (gensql.Environment, error)
	environmentGet             func(ctx context.Context, id uuid.UUID) (gensql.Environment, error)
	environmentIDByNames       func(ctx context.Context, arg gensql.EnvironmentIDByNamesParams) (uuid.UUID, error)
	environmentByNames         func(ctx context.Context, arg gensql.EnvironmentByNamesParams) (gensql.Environment, error)
	environmentUpdate          func(ctx context.Context, arg gensql.EnvironmentUpdateParams) (gensql.Environment, error)
	environmentsGet            func(ctx context.Context, tenantID uuid.UUID) ([]gensql.Environment, error)
	featureStateCreateOrUpdate func(ctx context.Context, arg gensql.FeatureStateCreateOrUpdateParams) (gensql.FeatureState, error)
	featureStateGet            func(ctx context.Context, arg gensql.FeatureStateGetParams) (gensql.FeatureState, error)
	featureStatesGet           func(ctx context.Context, environmentID uuid.UUID) ([]gensql.FeatureState, error)
	tenantCreate               func(ctx context.Context, arg gensql.TenantCreateParams) (gensql.Tenant, error)
	tenantGet                  func(ctx context.Context, id uuid.UUID) (gensql.Tenant, error)
	tenantGetByName            func(ctx context.Context, name string) (gensql.Tenant, error)
	tenantsGet                 func(ctx context.Context) ([]gensql.Tenant, error)
	tenantEnvironments         func(ctx context.Context) ([]gensql.TenantEnvironmentsRow, error)
	statusCreateOrUpdate       func(ctx context.Context, arg gensql.StatusCreateOrUpdateParams) error
	statusForEnvironment       func(ctx context.Context, environmentID uuid.UUID) ([]gensql.Status, error)
	statusForFeature           func(ctx context.Context, arg gensql.StatusForFeatureParams) (gensql.Status, error)
}

func (m *MockQuerier) ConfigDelete(ctx context.Context, id uuid.UUID) error {
	if m.configDelete == nil {
		panic("not implemented")
	}
	return m.configDelete(ctx, id)
}

func (m *MockQuerier) ConfigForEnv(ctx context.Context, arg gensql.ConfigForEnvParams) ([]gensql.ConfigForEnvRow, error) {
	if m.configForEnv == nil {
		panic("not implemented")
	}
	return m.configForEnv(ctx, arg)
}

func (m *MockQuerier) ConfigGet(ctx context.Context, feature string) ([]gensql.Configuration, error) {
	if m.configGet == nil {
		panic("not implemented")
	}
	return m.configGet(ctx, feature)
}

func (m *MockQuerier) ConfigGetForEnv(ctx context.Context, arg gensql.ConfigGetForEnvParams) ([]gensql.Configuration, error) {
	if m.configGetForEnv == nil {
		panic("not implemented")
	}
	return m.configGetForEnv(ctx, arg)
}

func (m *MockQuerier) ConfigUpdate(ctx context.Context, arg gensql.ConfigUpdateParams) (gensql.Configuration, error) {
	if m.configUpdate == nil {
		panic("not implemented")
	}
	return m.configUpdate(ctx, arg)
}

func (m *MockQuerier) ConfigUpdateOrCreate(ctx context.Context, arg gensql.ConfigUpdateOrCreateParams) (gensql.Configuration, error) {
	if m.configUpdateOrCreate == nil {
		panic("not implemented")
	}
	return m.configUpdateOrCreate(ctx, arg)
}

func (m *MockQuerier) EnvConfig(ctx context.Context, arg gensql.EnvConfigParams) ([]gensql.EnvConfigRow, error) {
	if m.envConfig == nil {
		panic("not implemented")
	}
	return m.envConfig(ctx, arg)
}

func (m *MockQuerier) EnvironmentCreate(ctx context.Context, arg gensql.EnvironmentCreateParams) (gensql.Environment, error) {
	if m.environmentCreate == nil {
		panic("not implemented")
	}
	return m.environmentCreate(ctx, arg)
}

func (m *MockQuerier) EnvironmentGet(ctx context.Context, id uuid.UUID) (gensql.Environment, error) {
	if m.environmentGet == nil {
		panic("not implemented")
	}
	return m.environmentGet(ctx, id)
}

func (m *MockQuerier) EnvironmentByNames(ctx context.Context, arg gensql.EnvironmentByNamesParams) (gensql.Environment, error) {
	if m.environmentByNames == nil {
		panic("not implemented")
	}
	return m.environmentByNames(ctx, arg)
}

func (m *MockQuerier) EnvironmentIDByNames(ctx context.Context, arg gensql.EnvironmentIDByNamesParams) (uuid.UUID, error) {
	if m.environmentIDByNames == nil {
		panic("not implemented")
	}
	return m.environmentIDByNames(ctx, arg)
}

func (m *MockQuerier) EnvironmentUpdate(ctx context.Context, arg gensql.EnvironmentUpdateParams) (gensql.Environment, error) {
	if m.environmentUpdate == nil {
		panic("not implemented")
	}
	return m.environmentUpdate(ctx, arg)
}

func (m *MockQuerier) EnvironmentsGet(ctx context.Context, tenantID uuid.UUID) ([]gensql.Environment, error) {
	if m.environmentsGet == nil {
		panic("not implemented")
	}
	return m.environmentsGet(ctx, tenantID)
}

func (m *MockQuerier) FeatureStateCreateOrUpdate(ctx context.Context, arg gensql.FeatureStateCreateOrUpdateParams) (gensql.FeatureState, error) {
	if m.featureStateCreateOrUpdate == nil {
		panic("not implemented")
	}
	return m.featureStateCreateOrUpdate(ctx, arg)
}

func (m *MockQuerier) FeatureStateGet(ctx context.Context, arg gensql.FeatureStateGetParams) (gensql.FeatureState, error) {
	if m.featureStateGet == nil {
		panic("not implemented")
	}
	return m.featureStateGet(ctx, arg)
}

func (m *MockQuerier) FeatureStatesGet(ctx context.Context, environmentID uuid.UUID) ([]gensql.FeatureState, error) {
	if m.featureStatesGet == nil {
		panic("not implemented")
	}
	return m.featureStatesGet(ctx, environmentID)
}

func (m *MockQuerier) TenantCreate(ctx context.Context, arg gensql.TenantCreateParams) (gensql.Tenant, error) {
	if m.tenantCreate == nil {
		panic("not implemented")
	}
	return m.tenantCreate(ctx, arg)
}

func (m *MockQuerier) TenantGet(ctx context.Context, id uuid.UUID) (gensql.Tenant, error) {
	if m.tenantGet == nil {
		panic("not implemented")
	}
	return m.tenantGet(ctx, id)
}

func (m *MockQuerier) TenantGetByName(ctx context.Context, name string) (gensql.Tenant, error) {
	if m.tenantGetByName == nil {
		panic("not implemented")
	}
	return m.tenantGetByName(ctx, name)
}

func (m *MockQuerier) TenantsGet(ctx context.Context) ([]gensql.Tenant, error) {
	if m.tenantGet == nil {
		panic("not implemented")
	}
	return m.tenantsGet(ctx)
}

func (m *MockQuerier) TenantEnvironments(ctx context.Context) ([]gensql.TenantEnvironmentsRow, error) {
	if m.tenantEnvironments == nil {
		panic("not implemented")
	}
	return m.tenantEnvironments(ctx)
}

func (m *MockQuerier) StatusCreateOrUpdate(ctx context.Context, arg gensql.StatusCreateOrUpdateParams) error {
	if m.statusCreateOrUpdate == nil {
		panic("not implemented")
	}
	return m.statusCreateOrUpdate(ctx, arg)
}

func (m *MockQuerier) StatusForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]gensql.Status, error) {
	if m.statusForEnvironment == nil {
		panic("not implemented")
	}
	return m.statusForEnvironment(ctx, environmentID)
}

func (m *MockQuerier) StatusForFeature(ctx context.Context, arg gensql.StatusForFeatureParams) (gensql.Status, error) {
	if m.statusForFeature == nil {
		panic("not implemented")
	}
	return m.statusForFeature(ctx, arg)
}

func (m *MockQuerier) WithTx(tx *sql.Tx) *gensql.Queries {
	panic("not implemented")
}
