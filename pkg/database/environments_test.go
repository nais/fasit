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

func TestRepo_EnvironmentGet(t *testing.T) {
	partnerID := uuid.New()
	id := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO partners (id, name) VALUES ('%s', 'test')", partnerID),
		fmt.Sprintf("INSERT INTO environments (id, name, description, partner_id, kind) VALUES ('%s', 'testname', 'testdesc', '%s', 'partner')", id, partnerID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	got, err := repo.EnvironmentGet(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Environment{
		ID:          id,
		Name:        "testname",
		Description: stringToPtr("testdesc"),
		Kind:        model.EnvironmentKindPartner,
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_EnvironmentsGet(t *testing.T) {
	partnerID := uuid.New()
	ids := []uuid.UUID{uuid.New(), uuid.New()}
	qs := []string{
		fmt.Sprintf("INSERT INTO partners (id, name) VALUES ('%s', 'test')", partnerID),
	}
	for i, id := range ids {
		qs = append(qs,
			fmt.Sprintf("INSERT INTO environments (id, name, description, partner_id, kind) VALUES ('%s', 'testname%v', 'testdesc', '%s', 'management')", id, i, partnerID),
		)
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	got, err := repo.EnvironmentsGet(context.Background(), partnerID)
	if err != nil {
		t.Fatal(err)
	}

	want := []*model.Environment{}

	for i, id := range ids {
		want = append(want, &model.Environment{
			ID:          id,
			Name:        fmt.Sprintf("testname%v", i),
			Description: stringToPtr("testdesc"),
			Kind:        model.EnvironmentKindManagement,
		})
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_EnvironmentCreate(t *testing.T) {
	partnerID := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO partners (id, name) VALUES ('%s', 'test')", partnerID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	create := &model.EnvironmentCreate{
		Name:        "somename",
		Description: stringToPtr("somedesc"),
		PartnerID:   partnerID,
		Kind:        model.EnvironmentKindPartner,
	}
	got, err := repo.EnvironmentCreate(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}

	want := &model.Environment{
		Name:        "somename",
		Description: stringToPtr("somedesc"),
		Kind:        model.EnvironmentKindPartner,
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "ID", "Created", "LastModified")
	if !cmp.Equal(want, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, got, opts))
	}
}

func TestRepo_EnvironmentUpdate(t *testing.T) {
	partnerID := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO partners (id, name) VALUES ('%s', 'test')", partnerID),
	}
	repo := newTestRepo(t, qs...)
	defer repo.Close()

	create := &model.EnvironmentCreate{
		Name:        "somename",
		Description: stringToPtr("somedesc"),
		PartnerID:   partnerID,
		Kind:        model.EnvironmentKindPartner,
	}
	env, err := repo.EnvironmentCreate(context.Background(), create)
	if err != nil {
		t.Fatal(err)
	}

	res, err := repo.EnvironmentUpdate(context.Background(), env.ID, &model.EnvironmentUpdate{
		Description: stringToPtr("somedesc2"),
	})
	want := &model.Environment{
		Name:        "somename",
		Description: stringToPtr("somedesc2"),
		Kind:        model.EnvironmentKindPartner,
	}

	opts := cmpopts.IgnoreFields(model.Environment{}, "ID", "Created", "LastModified")
	if !cmp.Equal(want, res, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want, res, opts))
	}

	got, err := repo.EnvironmentGet(context.Background(), env.ID)
	want2 := &model.Environment{
		Name:        "somename",
		Description: stringToPtr("somedesc2"),
		Kind:        model.EnvironmentKindPartner,
	}

	if !cmp.Equal(want2, got, opts) {
		t.Errorf("diff -want +got:\n%v", cmp.Diff(want2, got, opts))
	}
}

func TestRepo_EnvironmentIDByNames(t *testing.T) {
	partnerID := uuid.New()
	id := uuid.New()
	qs := []string{
		fmt.Sprintf("INSERT INTO partners (id, name) VALUES ('%s', 'test')", partnerID),
		fmt.Sprintf("INSERT INTO environments (id, name, description, partner_id) VALUES ('%s', 'testname', 'testdesc', '%s')", id, partnerID),
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
