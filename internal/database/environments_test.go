//go:build integration_test

package database

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
)

func TestRepo_EnvironmentGet(t *testing.T) {
	tenantID := uuid.New()
	id := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, description, tenant_id, kind) VALUES ('%s', 'testname', 'testdesc', '%s', 'tenant')", id, tenantID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	got, err := repo.EnvironmentGet(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Environment{
		ID:          id,
		TenantID:    tenantID,
		Name:        "testname",
		Description: new("testdesc"),
		Kind:        model.EnvironmentKindTenant,
		Reconcile:   true,
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_EnvironmentsGet(t *testing.T) {
	tenantID := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test')", tenantID),
	}
	for i, id := range ids {
		qs = append(qs,
			fmt.Sprintf("INSERT INTO environments (id, name, description, tenant_id, kind) VALUES ('%s', 'testname%v', 'testdesc', '%s', 'management')", id, i, tenantID),
		)
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	got, err := repo.EnvironmentsGet(context.Background(), tenantID)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.Environment{}

	for i, id := range ids {
		want = append(want, &model.Environment{
			ID:          id,
			TenantID:    tenantID,
			Name:        fmt.Sprintf("testname%v", i),
			Description: new("testdesc"),
			Kind:        model.EnvironmentKindManagement,
			Reconcile:   true,
		})
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_EnvironmentCreate(t *testing.T) {
	tenantID := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test')", tenantID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := setupContext(context.Background(), pool)

	create := &model.EnvironmentCreate{
		Name:        "somename",
		Description: new("somedesc"),
		TenantID:    tenantID,
		Kind:        model.EnvironmentKindTenant,
	}
	got, err := repo.EnvironmentCreate(ctx, create)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Environment{
		TenantID:    tenantID,
		Name:        "somename",
		Description: new("somedesc"),
		Kind:        model.EnvironmentKindTenant,
		Reconcile:   true,
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "ID", "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_EnvironmentUpdate(t *testing.T) {
	tenantID := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test')", tenantID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	ctx := setupContext(context.Background(), pool)

	create := &model.EnvironmentCreate{
		Name:        "somename",
		Description: new("somedesc"),
		TenantID:    tenantID,
		Kind:        model.EnvironmentKindTenant,
	}
	env, err := repo.EnvironmentCreate(ctx, create)
	if err != nil {
		t.Fatal(err)
	}

	res, err := repo.EnvironmentUpdate(ctx, env.ID, &model.EnvironmentUpdate{
		Description: new("somedesc2"),
	})
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Environment{
		TenantID:    tenantID,
		Name:        "somename",
		Description: new("somedesc2"),
		Kind:        model.EnvironmentKindTenant,
		Reconcile:   true,
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "ID", "Created", "LastModified")
	if !cmp.Equal(want, res, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, res, opts))
	}

	got, err := repo.EnvironmentGet(ctx, env.ID)
	if err != nil {
		t.Fatal(err)
	}
	want2 := &model.Environment{
		TenantID:    tenantID,
		Name:        "somename",
		Description: new("somedesc2"),
		Kind:        model.EnvironmentKindTenant,
		Reconcile:   true,
	}

	if !cmp.Equal(want2, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want2, got, opts))
	}
}

func TestRepo_EnvironmentIDByNames(t *testing.T) {
	tenantID := uuid.New()
	id := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO tenants (id, name) VALUES ('%s', 'test')", tenantID),
		fmt.Sprintf("INSERT INTO environments (id, name, description, tenant_id) VALUES ('%s', 'testname', 'testdesc', '%s')", id, tenantID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	got, err := repo.EnvironmentIDByNames(context.Background(), "test", "testname")
	if err != nil {
		t.Fatal(err)
	}

	if got != id {
		t.Errorf("got %v, want %v", got, id)
	}
}
