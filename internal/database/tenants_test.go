//go:build integration_test

package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/fasit/internal/graph/model"
	"k8s.io/utils/ptr"
)

func Test_repo_TenantCreate(t *testing.T) {
	repo := newTestRepo(t)
	defer repo.Close()

	ctx := context.Background()

	_, err := repo.TenantCreate(ctx, &model.TenantCreate{
		Name:        "test-name",
		Description: ptr.To("description"),
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
		Description: ptr.To("description"),
	})
	if err != nil {
		t.Fatalf("TenantCreate(ctx, tenant) err = %v, want nil", err)
	}

	got, err := repo.TenantGet(ctx, p.ID)
	if err != nil {
		t.Fatalf("TenantGet(ctx, id) err = %v, want nil", err)
	}

	want := &model.Tenant{
		ID:               p.ID,
		Name:             p.Name,
		Description:      p.Description,
		UpgradeDelayDays: 0,
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
