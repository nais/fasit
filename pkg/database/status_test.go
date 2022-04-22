//go:build integration_test

package database

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

func Test_repo_StatusCreateOrUpdate_Get(t *testing.T) {
	repo := newTestRepo(t, "status_create_or_update")

	_, env := partnerWithEnv(t, repo)
	_, env2 := partnerWithEnv(t, repo)

	tests := []struct {
		name  string
		envID uuid.UUID
		args  *message.Helm
		want  []*model.Status
	}{
		{
			name:  "create env1",
			envID: env.ID,
			args: &message.Helm{
				Name:          "test",
				Version:       "1.0.0",
				RolloutStatus: "failure",
				ConfigHash:    "12345",
			},
			want: []*model.Status{
				{
					EnvironmentID: env.ID,
					Feature:       "test",
					Version:       "1.0.0",
					Status:        "failure",
					ConfigHash:    "12345",
				},
			},
		},
		{
			name:  "update env1",
			envID: env.ID,
			args: &message.Helm{
				Name:          "test",
				Version:       "1.0.0",
				RolloutStatus: "success",
				ConfigHash:    "12345",
			},
			want: []*model.Status{
				{
					EnvironmentID: env.ID,
					Feature:       "test",
					Version:       "1.0.0",
					Status:        "success",
					ConfigHash:    "12345",
				},
			},
		},
		{
			name:  "update env2",
			envID: env2.ID,
			args: &message.Helm{
				Name:          "test",
				Version:       "1.0.0",
				RolloutStatus: "success",
				ConfigHash:    "12345",
			},
			want: []*model.Status{
				{
					EnvironmentID: env2.ID,
					Feature:       "test",
					Version:       "1.0.0",
					Status:        "success",
					ConfigHash:    "12345",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := repo.StatusCreateOrUpdate(context.Background(), tt.envID, tt.args); err != nil {
				t.Fatalf("StatusCreateOrUpdate() err = %v", err)
			}

			got, err := repo.StatusForEnvironment(context.Background(), tt.envID)
			if err != nil {
				t.Fatalf("StatusForEnvironment() err = %v", err)
			}

			opts := cmpopts.IgnoreFields(model.Status{}, "Created", "LastModified")
			if diff := cmp.Diff(tt.want, got, opts); diff != "" {
				t.Errorf("diff -want +got:\n%v", diff)
			}
		})
	}
}
