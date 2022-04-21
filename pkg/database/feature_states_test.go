package database

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
)

func TestRepo_FeatureStateCreateOrUpdate(t *testing.T) {
	tests := map[string]struct {
		featureState gensql.FeatureState
		expectError  bool
	}{
		"success": {
			featureState: gensql.FeatureState{
				EnvironmentID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Feature:       "banan",
				Enabled:       true,
			},
			expectError: false,
		},
		"missing": {
			featureState: gensql.FeatureState{
				EnvironmentID: uuid.MustParse("00000000-0000-0000-0000-000000000001"),
				Feature:       "banan",
				Enabled:       false,
			},
			expectError: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mq := &MockQuerier{
				featureStatesGet: func(ctx context.Context, uuid2 uuid.UUID) ([]gensql.FeatureState, error) {
					return []gensql.FeatureState{tc.featureState}, nil
				},
				featureStateCreateOrUpdate: func(ctx context.Context, arg gensql.FeatureStateCreateOrUpdateParams) (gensql.FeatureState, error) {
					return gensql.FeatureState{}, nil
				},
			}

			repo := repo{querier: mq}

			f := &feature.Feature{
				Name:      "kake",
				DependsOn: []string{"banan"},
			}

			_, err := repo.FeatureStatesCreateOrUpdate(context.Background(), uuid.New(), f, true)
			if (err != nil) != tc.expectError {
				t.Errorf("expected error %v, got %v", tc.expectError, err)
			}
		})
	}
}
