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

func Test_repo_TenantCreate(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	_, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "test-name",
		Description: stringToPtr("description"),
	})
	if err != nil {
		t.Fatalf("TenantCreate(ctx, tenant) err = %v, want nil", err)
	}
}

func Test_repo_TenantGet(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	p, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "test-name",
		Description: stringToPtr("description"),
	})
	if err != nil {
		t.Fatalf("TenantCreate(ctx, tenant) err = %v, want nil", err)
	}

	got, err := repo.TenantGet(ctx, p.ID)
	if err != nil {
		t.Fatalf("TenantGet(ctx, id) err = %v, want nil", err)
	}

	want := &model.Tenant{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
	}

	opts := cmpopts.IgnoreFields(model.Tenant{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func Test_repo_TenantsGet(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	want := []string{}
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("test-name-%d", i)
		want = append(want, name)
		_, err := repo.TenantCreate(ctx, &model.TenantCreate{
			Name: name,
		})
		if err != nil {
			t.Fatalf("TenantCreate(ctx, tenant) err = %v, want nil", err)
		}
	}

	p2, err := repo.TenantsGet(ctx)
	if err != nil {
		t.Fatalf("TenantGet(ctx, id) err = %v, want nil", err)
	}

	got := []string{}
	for _, p := range p2 {
		got = append(got, p.Name)
	}

	if !cmp.Equal(want, got) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got))
	}
}

func TestRepo_TenantEnvironments(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	eids := []uuid.UUID{}
	tenantIDs := []uuid.UUID{}
	for i := 0; i < 2; i++ {
		p, err := repo.TenantCreate(ctx, &model.TenantCreate{
			Name: fmt.Sprintf("test-tenant-%v", i),
		})
		if err != nil {
			t.Fatalf("TenantCreate(ctx, tenant) err = %v, want nil", err)
		}
		tenantIDs = append(tenantIDs, p.ID)

		for j := 0; j < 2; j++ {
			e, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
				Name:        fmt.Sprintf("test-env-%v", j),
				Description: stringToPtr("description"),
				TenantID:    p.ID,
				Kind:        model.EnvironmentKindTenant,
			})
			if err != nil {
				t.Fatalf("EnvironmentCreate(ctx, env) err = %v, want nil", err)
			}
			eids = append(eids, e.ID)
		}
	}

	got, err := repo.TenantEnvironments(ctx)
	if err != nil {
		t.Fatalf("TenantEnvironments(ctx) err = %v, want nil", err)
	}

	want := []*model.TenantEnvironments{
		{
			Environment: model.Environment{
				ID:          eids[0],
				Name:        "test-env-0",
				Description: stringToPtr("description"),
				Kind:        model.EnvironmentKindTenant,
			},
			TenantName: "test-tenant-0",
			TenantID:   tenantIDs[0],
		},
		{
			Environment: model.Environment{
				ID:          eids[1],
				Name:        "test-env-1",
				Description: stringToPtr("description"),
				Kind:        model.EnvironmentKindTenant,
			},
			TenantName: "test-tenant-0",
			TenantID:   tenantIDs[0],
		},
		{
			Environment: model.Environment{
				ID:          eids[2],
				Name:        "test-env-0",
				Description: stringToPtr("description"),
				Kind:        model.EnvironmentKindTenant,
			},
			TenantName: "test-tenant-1",
			TenantID:   tenantIDs[1],
		},
		{
			Environment: model.Environment{
				ID:          eids[3],
				Name:        "test-env-1",
				Description: stringToPtr("description"),
				Kind:        model.EnvironmentKindTenant,
			},
			TenantName: "test-tenant-1",
			TenantID:   tenantIDs[1],
		},
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func tenantWithEnv(t *testing.T, name string, repo Repo) (*model.Tenant, *model.Environment) {
	t.Helper()

	ctx := context.Background()
	p, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name: name,
	})
	if err != nil {
		t.Fatalf("TenantCreate(ctx, tenant) err = %v, want nil", err)
	}

	env, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
		Name:     "test-env",
		TenantID: p.ID,
		Kind:     model.EnvironmentKindTenant,
	})
	if err != nil {
		t.Fatalf("EnvironmentCreate(ctx, env) err = %v, want nil", err)
	}

	return p, env
}
