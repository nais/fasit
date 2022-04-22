//go:build integration_test

package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/nais/fasit/pkg/graph/model"
)

func Test_repo_PartnerCreate(t *testing.T) {
	repo := newTestRepo(t, "partner_create")

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
	repo := newTestRepo(t, "partner_get")

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
	repo := newTestRepo(t, "partners_get")

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
