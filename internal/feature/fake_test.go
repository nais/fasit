package feature

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsqlfake"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/sirupsen/logrus/hooks/test"
)

// fakeQuerier implements featuresql.Querier for unit tests.
// Set function fields for methods your test exercises; unset fields panic.
type fakeQuerier struct {
	configGlobalDeleteFunc           func(ctx context.Context, id uuid.UUID) error
	configEnvGetByKeyFunc            func(ctx context.Context, arg featuresql.ConfigEnvGetByKeyParams) (featuresql.ConfigurationsEnvironment, error)
	configEnvUpsertFunc              func(ctx context.Context, arg featuresql.ConfigEnvUpsertParams) (featuresql.ConfigurationsEnvironment, error)
	configGlobalGetByIDFunc          func(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsGlobal, error)
	configGlobalGetByKeyFunc         func(ctx context.Context, arg featuresql.ConfigGlobalGetByKeyParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalUpsertFunc           func(ctx context.Context, arg featuresql.ConfigGlobalUpsertParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalUpdateFunc           func(ctx context.Context, arg featuresql.ConfigGlobalUpdateParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalListByFeatureFunc    func(ctx context.Context, feature string) ([]featuresql.ConfigurationsGlobal, error)
	configEnvListByFeatureAndEnvFunc func(ctx context.Context, arg featuresql.ConfigEnvListByFeatureAndEnvParams) ([]featuresql.ConfigurationsEnvironment, error)
}

func (f *fakeQuerier) GetLatestDeployedDeployInstruction(ctx context.Context, arg featuresql.GetLatestDeployedDeployInstructionParams) (featuresql.DeployInstruction, error) {
	panic("implement me")
}

func (f *fakeQuerier) GetLatestDeployInstruction(ctx context.Context, arg featuresql.GetLatestDeployInstructionParams) (featuresql.DeployInstruction, error) {
	panic("implement me")
}

func (f *fakeQuerier) GetPreviousDeployInstruction(ctx context.Context, id uuid.UUID) (featuresql.DeployInstruction, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigGlobalDelete(ctx context.Context, id uuid.UUID) error {
	return f.configGlobalDeleteFunc(ctx, id)
}

func (f *fakeQuerier) ConfigEnvDelete(ctx context.Context, id uuid.UUID) error {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigEnvGetByKey(ctx context.Context, arg featuresql.ConfigEnvGetByKeyParams) (featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvGetByKeyFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigEnvGetByID(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsEnvironment, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigEnvListByFeature(ctx context.Context, feature string) ([]featuresql.ConfigurationsEnvironment, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigEnvUpdate(ctx context.Context, arg featuresql.ConfigEnvUpdateParams) (featuresql.ConfigurationsEnvironment, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ConfigEnvUpsert(ctx context.Context, arg featuresql.ConfigEnvUpsertParams) (featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvUpsertFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigEnvListByFeatureAndEnv(ctx context.Context, arg featuresql.ConfigEnvListByFeatureAndEnvParams) ([]featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvListByFeatureAndEnvFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigGlobalListByFeature(ctx context.Context, feature string) ([]featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalListByFeatureFunc(ctx, feature)
}

func (f *fakeQuerier) ConfigGlobalGetByID(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalGetByIDFunc(ctx, id)
}

func (f *fakeQuerier) ConfigGlobalGetByKey(ctx context.Context, arg featuresql.ConfigGlobalGetByKeyParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalGetByKeyFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigGlobalUpsert(ctx context.Context, arg featuresql.ConfigGlobalUpsertParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalUpsertFunc(ctx, arg)
}

func (f *fakeQuerier) ConfigGlobalUpdate(ctx context.Context, arg featuresql.ConfigGlobalUpdateParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalUpdateFunc(ctx, arg)
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

func (f *fakeQuerier) FeatureDataCreate(context.Context, featuresql.FeatureDataCreateParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureNames(context.Context) ([]string, error) {
	panic("not implemented")
}

func (f *fakeQuerier) LatestFeatureData(context.Context, string) (featuresql.LatestFeatureDataRow, error) {
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
func newTestCtx(t *testing.T) (context.Context, *fakeQuerier, *auditsqlfake.Querier) {
	t.Helper()
	log, _ := test.NewNullLogger()
	fq := &fakeQuerier{}
	aq := &auditsqlfake.Querier{}
	ctx := context.WithValue(context.Background(), QuerierKey, featuresql.Querier(fq))
	ctx = audit.RegisterTestDeps(ctx, aq, log)
	return ctx, fq, aq
}
