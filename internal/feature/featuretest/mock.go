package featuretest

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/environment/environmenttest"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func OnFeatureByName(ctx context.Context, name string, expect *model.Feature) {
	GetQuerier(ctx).On("FeatureByName", ctx, name).Return(featuresql.FeatureByNameRow{
		FeatureDatum: modelToFeatureDatum(expect),
	}, nil).Once()
}

func OnConfigGet(ctx context.Context, name string, config []*model.Configuration) {
	ret := make([]featuresql.ConfigurationsGlobal, len(config))
	for i, c := range config {
		ret[i] = featuresql.ConfigurationsGlobal{
			ID:      c.ID,
			Feature: name,
			Key:     c.Key,
			Value:   c.Content,
			Created: pgtype.Timestamptz{
				Time:  c.Created,
				Valid: true,
			},
		}
	}
	GetQuerier(ctx).EXPECT().ConfigGet(mock.Anything, name).Return(ret, nil).Once()
}

func OnEnvConfig(ctx context.Context, envID uuid.UUID, ret []featuresql.EnvConfigRow) {
	featureMock := GetQuerier(ctx)
	featureMock.EXPECT().EnvConfig(mock.Anything, mock.MatchedBy(func(params featuresql.EnvConfigParams) bool {
		return params.EnvironmentID == envID
	})).Return(ret, nil).Once()
}

func OnMappingValuesForEnvironment(ctx context.Context, envID, tenantID uuid.UUID, showSensitive bool, expected *feature.ComputedValues) {
	environmenttest.GetQuerier(ctx).EXPECT().Get(ctx, envID).Return(
		environmentsql.Environment{
			ID:       envID,
			TenantID: tenantID,
			Name:     "env-name",
			Kind:     environmentsql.EnvironmentKind(expected.Kind),
		}, nil,
	).Once()
	environmenttest.GetQuerier(ctx).EXPECT().GetTenant(ctx, tenantID).Return(
		environmentsql.Tenant{
			ID:   tenantID,
			Name: "tenant-name",
		}, nil,
	).Once()

	values, err := json.Marshal(expected.Env)
	if err != nil {
		panic(err)
	}

	ret := []featuresql.ListMappingValuesForTenantRow{
		{
			ID:     envID,
			Name:   "env-name",
			Kind:   featuresql.EnvironmentKind(expected.Kind),
			Values: values,
		},
	}

	GetQuerier(ctx).EXPECT().ListMappingValuesForTenant(ctx, mock.MatchedBy(func(params featuresql.ListMappingValuesForTenantParams) bool {
		return params.Tenantid == tenantID && params.Showsensitive == showSensitive
	})).Return(ret, nil).Once()
}

func OnFeatureByNameForEnv(ctx context.Context, envName string, expect *model.Feature) {
	featureMock := GetQuerier(ctx)
	featureMock.On("GetEnvironmentFeature", ctx, mock.Anything).Return(featuresql.GetEnvironmentFeatureRow{}, pgx.ErrNoRows).Once()
	environmenttest.GetQuerier(ctx).On("Get", ctx, mock.Anything).Return(
		environmentsql.Environment{
			ID:   uuid.New(),
			Name: envName,
		}, nil,
	).Once()
	featureMock.On("FeatureByName", ctx, expect.Name).Return(featuresql.FeatureByNameRow{
		FeatureDatum: modelToFeatureDatum(expect),
	}, nil).Once()
}

func OnFeaturesForKind(ctx context.Context, kind model.EnvironmentKind, expect []*model.Feature) {
	ret := make([]featuresql.FeaturesForKindRow, len(expect))
	for i, f := range expect {
		ret[i] = featuresql.FeaturesForKindRow{
			FeatureDatum:   modelToFeatureDatum(f),
			Hasdeployments: f.HasDeployments,
		}
	}
	GetQuerier(ctx).On("FeaturesForKind", mock.Anything, kind.String()).Return(ret, nil).Once()
}

func OnFeatureStatesGet(ctx context.Context, envID uuid.UUID, expect []*model.FeatureState) {
	ret := make([]featuresql.FeatureStatesGetRow, len(expect))
	for i, f := range expect {
		enabledAt := pgtype.Timestamptz{}
		if f.EnabledAt != nil {
			enabledAt.Time = *f.EnabledAt
			enabledAt.Valid = true
		}

		ret[i] = featuresql.FeatureStatesGetRow{
			EnvironmentID: envID,
			Name:          f.FeatureName,
			Enabled:       f.Enabled,
			Created: pgtype.Timestamptz{
				Time:  f.Created,
				Valid: true,
			},
			LastModified: pgtype.Timestamptz{
				Time:  f.LastModified,
				Valid: true,
			},
			EnabledAt: enabledAt,
		}
	}
	GetQuerier(ctx).On("FeatureStatesGet", mock.Anything, envID).Return(ret, nil).Once()
}

func OnHelmValues(ctx context.Context, envID uuid.UUID, featureName string, expect map[string]interface{}) context.Context {
	helmValuesFunc := func(ctx context.Context, f *model.Feature, envID uuid.UUID) (map[string]any, error) {
		return expect, nil
	}
	ctx = context.WithValue(ctx, feature.HelmValuesFuncKey, helmValuesFunc)
	return ctx
}

func modelToFeatureDatum(f *model.Feature) featuresql.FeatureDatum {
	values, err := json.Marshal(f.FeatureYAML.Values)
	if err != nil {
		panic(err)
	}
	defaultVals, err := json.Marshal(f.ValuesYAML)
	if err != nil {
		panic(err)
	}
	deps, err := json.Marshal(f.FeatureYAML.Dependencies)
	if err != nil {
		panic(err)
	}
	kinds := make([]featuresql.EnvironmentKind, len(f.EnvironmentKinds))
	for i, k := range f.EnvironmentKinds {
		kinds[i] = featuresql.EnvironmentKind(k)
	}
	rename, err := json.Marshal(f.Rename)
	if err != nil {
		panic(err)
	}
	return featuresql.FeatureDatum{
		Name:          f.Name,
		Version:       f.Version,
		Chart:         f.Chart,
		Description:   f.Description,
		Source:        f.Source,
		Kinds:         kinds,
		Dependencies:  deps,
		Values:        values,
		DefaultValues: defaultVals,
		Timeout:       f.Timeout.Milliseconds(),
		Rename:        rename,
	}
}
