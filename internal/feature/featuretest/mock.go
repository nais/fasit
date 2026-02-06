package featuretest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/environment/environmentsql"
	"github.com/nais/fasit/internal/environment/environmenttest"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/mock"
)

func OnFeatureByName(ctx context.Context, t *testing.T, name string, expect *model.Feature) context.Context {
	ctx = RegisterMock(ctx, t)
	GetQuerier(ctx).On("FeatureByName", ctx, name).Return(featuresql.FeatureByNameRow{
		FeatureDatum: modelToFeatureDatum(expect),
	}, nil).Once()
	return ctx
}

func OnFeatureByNameForEnv(ctx context.Context, t *testing.T, envName string, expect *model.Feature) context.Context {
	ctx = RegisterMock(ctx, t)
	ctx = environmenttest.RegisterMock(ctx, t)
	featureMock := GetQuerier(ctx)
	envMock := environmenttest.GetQuerier(ctx)
	featureMock.On("GetEnvironmentFeature", ctx, mock.Anything).Return(featuresql.GetEnvironmentFeatureRow{}, pgx.ErrNoRows).Once()
	envMock.On("Get", ctx, mock.Anything).Return(
		environmentsql.Environment{
			ID:   uuid.New(),
			Name: envName,
		}, nil,
	).Once()
	featureMock.On("FeatureByName", ctx, expect.Name).Return(featuresql.FeatureByNameRow{
		FeatureDatum: modelToFeatureDatum(expect),
	}, nil).Once()
	return ctx
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
