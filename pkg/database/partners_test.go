//go:build integration_test

package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
)

func Test_repo_PartnerCreate(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	_, err := repo.PartnerCreate(ctx, &model.PartnerCreate{
		Name:        "test-name",
		Description: stringToPtr("description"),
	})
	if err != nil {
		t.Fatalf("PartnerCreate(ctx, partner) err = %v, want nil", err)
	}
}

func Test_repo_PartnerGet(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	p, err := repo.PartnerCreate(ctx, &model.PartnerCreate{
		Name:        "test-name",
		Description: stringToPtr("description"),
	})
	if err != nil {
		t.Fatalf("PartnerCreate(ctx, partner) err = %v, want nil", err)
	}

	got, err := repo.PartnerGet(ctx, p.ID)
	if err != nil {
		t.Fatalf("PartnerGet(ctx, id) err = %v, want nil", err)
	}

	want := &model.Partner{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
	}

	opts := cmpopts.IgnoreFields(model.Partner{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func Test_repo_PartnersGet(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	want := []string{}
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("test-name-%d", i)
		want = append(want, name)
		_, err := repo.PartnerCreate(ctx, &model.PartnerCreate{
			Name: name,
		})
		if err != nil {
			t.Fatalf("PartnerCreate(ctx, partner) err = %v, want nil", err)
		}
	}

	p2, err := repo.PartnersGet(ctx)
	if err != nil {
		t.Fatalf("PartnerGet(ctx, id) err = %v, want nil", err)
	}

	got := []string{}
	for _, p := range p2 {
		got = append(got, p.Name)
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_PartnerEnvironments(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	eids := []uuid.UUID{}
	for i := 0; i < 2; i++ {
		p, err := repo.PartnerCreate(ctx, &model.PartnerCreate{
			Name: fmt.Sprintf("test-partner-%v", i),
		})
		if err != nil {
			t.Fatalf("PartnerCreate(ctx, partner) err = %v, want nil", err)
		}

		for j := 0; j < 2; j++ {
			e, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
				Name:        fmt.Sprintf("test-env-%v", j),
				Description: stringToPtr("description"),
				PartnerID:   p.ID,
			})
			if err != nil {
				t.Fatalf("EnvironmentCreate(ctx, env) err = %v, want nil", err)
			}
			eids = append(eids, e.ID)
		}
	}

	got, err := repo.PartnerEnvironments(ctx)
	if err != nil {
		t.Fatalf("PartnerEnvironments(ctx) err = %v, want nil", err)
	}

	want := []*model.PartnerEnvironments{
		{
			Environment: model.Environment{
				ID:          eids[0],
				Name:        "test-env-0",
				Description: stringToPtr("description"),
			},
			PartnerName: "test-partner-0",
		},
		{
			Environment: model.Environment{
				ID:          eids[1],
				Name:        "test-env-1",
				Description: stringToPtr("description"),
			},
			PartnerName: "test-partner-0",
		},
		{
			Environment: model.Environment{
				ID:          eids[2],
				Name:        "test-env-0",
				Description: stringToPtr("description"),
			},
			PartnerName: "test-partner-1",
		},
		{
			Environment: model.Environment{
				ID:          eids[3],
				Name:        "test-env-1",
				Description: stringToPtr("description"),
			},
			PartnerName: "test-partner-1",
		},
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func partnerWithEnv(t *testing.T, repo Repo) (*model.Partner, *model.Environment) {
	t.Helper()

	ctx := context.Background()
	p, err := repo.PartnerCreate(ctx, &model.PartnerCreate{
		Name: "test-partner",
	})
	if err != nil {
		t.Fatalf("PartnerCreate(ctx, partner) err = %v, want nil", err)
	}

	env, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:      "test-env",
		PartnerID: p.ID,
	})
	if err != nil {
		t.Fatalf("EnvironmentCreate(ctx, env) err = %v, want nil", err)
	}

	return p, env
}
