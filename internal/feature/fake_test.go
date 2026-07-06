package feature

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsqlfake"
	"github.com/nais/fasit/internal/feature/featuresql"
)

// fakeQuerier implements featuresql.Querier for unit tests.
// Set function fields for methods your test exercises; unset fields panic.
type fakeQuerier struct {
	configGlobalDeleteFunc        func(ctx context.Context, id uuid.UUID) error
	configEnvGetByKeyFunc         func(ctx context.Context, arg featuresql.GetEnvConfigByKeyParams) (featuresql.ConfigurationsEnvironment, error)
	configEnvUpsertFunc           func(ctx context.Context, arg featuresql.UpsertEnvConfigParams) (featuresql.ConfigurationsEnvironment, error)
	configGlobalGetByIDFunc       func(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsGlobal, error)
	configGlobalGetByKeyFunc      func(ctx context.Context, arg featuresql.GetGlobalConfigByKeyParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalUpsertFunc        func(ctx context.Context, arg featuresql.UpsertGlobalConfigParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalUpdateFunc        func(ctx context.Context, arg featuresql.UpdateGlobalConfigParams) (featuresql.ConfigurationsGlobal, error)
	configGlobalListByFeatureFunc func(ctx context.Context, feature string) ([]featuresql.ConfigurationsGlobal, error)
	configEnvListByFeatureFunc    func(ctx context.Context, arg featuresql.ListEnvConfigByFeatureParams) ([]featuresql.ConfigurationsEnvironment, error)
}

func (f *fakeQuerier) ListEnvConfigByFeature(ctx context.Context, arg featuresql.ListEnvConfigByFeatureParams) ([]featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvListByFeatureFunc(ctx, arg)
}

func (f *fakeQuerier) ListGlobalConfigByFeature(ctx context.Context, feature string) ([]featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalListByFeatureFunc(ctx, feature)
}

func (f *fakeQuerier) GetLatestDeployInstruction(ctx context.Context, arg featuresql.GetLatestDeployInstructionParams) (featuresql.GetLatestDeployInstructionRow, error) {
	panic("implement me")
}

func (f *fakeQuerier) ListDeployLog(ctx context.Context, arg featuresql.ListDeployLogParams) ([]featuresql.ListDeployLogRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListLatestDeployInstructionsForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]featuresql.ListLatestDeployInstructionsForEnvironmentRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListLatestDeployedForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]featuresql.ListLatestDeployedForEnvironmentRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListLatestDeployInstructionsForFeature(ctx context.Context, featureName string) ([]featuresql.ListLatestDeployInstructionsForFeatureRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListLatestDeployedForFeature(ctx context.Context, featureName string) ([]featuresql.ListLatestDeployedForFeatureRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) DeleteGlobalConfig(ctx context.Context, id uuid.UUID) error {
	return f.configGlobalDeleteFunc(ctx, id)
}

func (f *fakeQuerier) DeleteEnvConfig(ctx context.Context, id uuid.UUID) error {
	panic("not implemented")
}

func (f *fakeQuerier) GetEnvConfigByKey(ctx context.Context, arg featuresql.GetEnvConfigByKeyParams) (featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvGetByKeyFunc(ctx, arg)
}

func (f *fakeQuerier) GetEnvConfigByID(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsEnvironment, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListAllEnvConfigByFeature(ctx context.Context, feature string) ([]featuresql.ConfigurationsEnvironment, error) {
	panic("not implemented")
}

func (f *fakeQuerier) UpdateEnvConfig(ctx context.Context, arg featuresql.UpdateEnvConfigParams) (featuresql.ConfigurationsEnvironment, error) {
	panic("not implemented")
}

func (f *fakeQuerier) UpsertEnvConfig(ctx context.Context, arg featuresql.UpsertEnvConfigParams) (featuresql.ConfigurationsEnvironment, error) {
	return f.configEnvUpsertFunc(ctx, arg)
}

func (f *fakeQuerier) GetGlobalConfigByID(ctx context.Context, id uuid.UUID) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalGetByIDFunc(ctx, id)
}

func (f *fakeQuerier) GetGlobalConfigByKey(ctx context.Context, arg featuresql.GetGlobalConfigByKeyParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalGetByKeyFunc(ctx, arg)
}

func (f *fakeQuerier) UpsertGlobalConfig(ctx context.Context, arg featuresql.UpsertGlobalConfigParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalUpsertFunc(ctx, arg)
}

func (f *fakeQuerier) UpdateGlobalConfig(ctx context.Context, arg featuresql.UpdateGlobalConfigParams) (featuresql.ConfigurationsGlobal, error) {
	return f.configGlobalUpdateFunc(ctx, arg)
}

func (f *fakeQuerier) DeleteDisabledFeature(context.Context, featuresql.DeleteDisabledFeatureParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) GetDisabledFeature(context.Context, featuresql.GetDisabledFeatureParams) (featuresql.DisabledFeature, error) {
	panic("not implemented")
}

func (f *fakeQuerier) SetDisabledFeature(context.Context, featuresql.SetDisabledFeatureParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) ListDisabledFeaturesByEnvironment(context.Context, uuid.UUID) ([]featuresql.DisabledFeature, error) {
	panic("not implemented")
}

func (f *fakeQuerier) ListDisabledFeatureEnvironments(context.Context, string) ([]featuresql.ListDisabledFeatureEnvironmentsRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureDataCreate(context.Context, featuresql.FeatureDataCreateParams) error {
	panic("not implemented")
}

func (f *fakeQuerier) ListActiveFeatures(context.Context) ([]featuresql.ListActiveFeaturesRow, error) {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureNames(context.Context) ([]string, error) {
	panic("not implemented")
}

func (f *fakeQuerier) FeatureDataByVersion(context.Context, featuresql.FeatureDataByVersionParams) (featuresql.FeatureDataByVersionRow, error) {
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

var _ featuresql.Querier = (*fakeQuerier)(nil)

// setupTestCtx builds a context with fake feature and audit queriers.
// Returns the context, feature fake, and audit fake.
func newTestCtx(t *testing.T) (context.Context, *fakeQuerier, *auditsqlfake.Querier) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	fq := &fakeQuerier{}
	aq := &auditsqlfake.Querier{}
	ctx := context.WithValue(context.Background(), QuerierKey, featuresql.Querier(fq))
	ctx = audit.RegisterTestDeps(ctx, aq, log)
	return ctx, fq, aq
}
