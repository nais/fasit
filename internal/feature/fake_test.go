package feature

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsql/auditfake"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/sirupsen/logrus/hooks/test"
)

// fakeQuerier implements featuresql.Querier for unit tests.
// Set function fields for methods your test exercises; unset fields panic.
type fakeQuerier struct {
	configDeleteFunc               func(ctx context.Context, id uuid.UUID) error
	configEnvGetFunc               func(ctx context.Context, arg featuresql.ConfigEnvGetParams) (featuresql.ConfigurationsEnvironment, error)
	configEnvUpdateOrCreateFunc    func(ctx context.Context, arg featuresql.ConfigEnvUpdateOrCreateParams) (featuresql.ConfigurationsEnvironment, error)
	configGetByIDFunc              func(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsGlobal, error)
	configGlobalGetByKeyFunc       func(ctx context.Context, arg featuresql.ConfigGlobalGetByKeyParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalUpdateOrCreateFunc func(ctx context.Context, arg featuresql.ConfigGlobalUpdateOrCreateParams) (featuresql.ConfigurationsGlobal, error)
	configUpdateFunc               func(ctx context.Context, arg featuresql.ConfigUpdateParams) (featuresql.ConfigurationsGlobal, error)
	featureStateGetFunc            func(ctx context.Context, arg featuresql.FeatureStateGetParams) (featuresql.FeatureState, error)
	featureStateCreateOrUpdateFunc func(ctx context.Context, arg featuresql.FeatureStateCreateOrUpdateParams) (featuresql.FeatureState, error)
}

func (f *fakeQuerier) ConfigDelete(ctx context.Context, id uuid.UUID) error {
	return f.configDeleteFunc(ctx, id)
}

func (f *fakeQuerier) ConfigEnvGet(ctx context.Context, arg featuresql.ConfigEnvGetParams) (featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvGetFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigEnvUpdateOrCreate(ctx context.Context, arg featuresql.ConfigEnvUpdateOrCreateParams) (featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvUpdateOrCreateFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigForEnvironmentFilteredByKeys(context.Context, featuresql.ConfigForEnvironmentFilteredByKeysParams) ([]featuresql.ConfigForEnvironmentFilteredByKeysRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigGet(context.Context, string) ([]featuresql.ConfigurationsGlobal, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigGetByID(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
	return f.configGetByIDFunc(ctx, id)
}

func (f *fakeQuerier) ConfigGlobalGetByKey(ctx context.Context, arg featuresql.ConfigGlobalGetByKeyParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalGetByKeyFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigGlobalUpdateOrCreate(ctx context.Context, arg featuresql.ConfigGlobalUpdateOrCreateParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalUpdateOrCreateFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigOverridesByFeature(context.Context, string) ([]featuresql.ConfigOverridesByFeatureRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigRenameEnv(context.Context, featuresql.ConfigRenameEnvParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigRenameGlobal(context.Context, featuresql.ConfigRenameGlobalParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigUpdate(ctx context.Context, arg featuresql.ConfigUpdateParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configUpdateFunc(ctx, arg)
}

func (f *fakeQuerier) DisabledFeatureDelete(context.Context, featuresql.DisabledFeatureDeleteParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) DisabledFeatureGet(context.Context, featuresql.DisabledFeatureGetParams) (featuresql.DisabledFeature, error) {
	panic("not implemented")
}

func (f *fakeQuerier) DisabledFeatureSet(context.Context, featuresql.DisabledFeatureSetParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) DisabledFeaturesByEnvironment(context.Context, uuid.UUID) ([]featuresql.DisabledFeature, error) {
	panic("not implemented")
}

func (f *fakeQuerier) EnvConfig(context.Context, featuresql.EnvConfigParams) ([]featuresql.EnvConfigRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureByName(context.Context, string) (featuresql.FeatureByNameRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureDataCreate(context.Context, featuresql.FeatureDataCreateParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureStateCreateOrUpdate(ctx context.Context, arg featuresql.FeatureStateCreateOrUpdateParams) (featuresql.FeatureState, error) {
	return f.featureStateCreateOrUpdateFunc(ctx, arg)
}

func (f *fakeQuerier) FeatureStateGet(ctx context.Context, arg featuresql.FeatureStateGetParams) (featuresql.FeatureState, error) {
	return f.featureStateGetFunc(ctx, arg)
}

func (f *fakeQuerier) FeatureStatesGet(context.Context, uuid.UUID) ([]featuresql.FeatureStatesGetRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureVersionUpdate(context.Context, featuresql.FeatureVersionUpdateParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) Features(context.Context) ([]featuresql.FeaturesRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) GetEnvironmentFeature(context.Context, featuresql.GetEnvironmentFeatureParams) (featuresql.GetEnvironmentFeatureRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) HasMatchingDeployment(context.Context, featuresql.HasMatchingDeploymentParams) (bool, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListMappingValuesForTenant(context.Context, featuresql.ListMappingValuesForTenantParams) ([]featuresql.ListMappingValuesForTenantRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListSecretKeysForTenant(context.Context, uuid.UUID) ([]featuresql.ListSecretKeysForTenantRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) LogsByDeployInstruction(context.Context, uuid.UUID) ([]featuresql.Log, error) {
	panic("not implemented")
}

func (f *fakeQuerier) LogsByID(context.Context, int64) (featuresql.Log, error) {
	panic("not implemented")
}

func (f *fakeQuerier) LogsCreate(context.Context, []featuresql.LogsCreateParams) *featuresql.LogsCreateBatchResults {
	panic("not implemented")
}

var _ featuresql.Querier = (*fakeQuerier)(nil)

// setupTestCtx builds a context with fake feature and audit queriers.
// Returns the context, feature fake, and audit fake.
func newTestCtx(t *testing.T) (context.Context, *fakeQuerier, *auditfake.Querier) {
	t.Helper()
	log, _ := test.NewNullLogger()
	fq := &fakeQuerier{}
	aq := &auditfake.Querier{}
	ctx := context.WithValue(context.Background(), QuerierKey, featuresql.Querier(fq))
	ctx = audit.RegisterTestDeps(ctx, aq, log)
	return ctx, fq, aq
}
