package audit_test

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/audit/auditsqlfake"
)

func TestAssignmentCreators(t *testing.T) {
	firstID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	secondID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	q := &auditsqlfake.Querier{
		AuditListAssignmentCreatorsFunc: func(_ context.Context, _ []string) ([]auditsql.ListAssignmentCreatorsRow, error) {
			return []auditsql.ListAssignmentCreatorsRow{
				{AssignmentID: firstID.String(), Actor: "first@example.com"},
				{AssignmentID: secondID.String(), Actor: "second@example.com"},
			}, nil
		},
	}
	ctx := audit.RegisterTestDeps(context.Background(), q, slog.New(slog.NewTextHandler(io.Discard, nil)))

	got, err := audit.AssignmentCreators(ctx, []uuid.UUID{firstID, secondID})
	if err != nil {
		t.Fatalf("AssignmentCreators() error = %v", err)
	}
	if got[firstID] != "first@example.com" {
		t.Errorf("creator for %s = %q, want %q", firstID, got[firstID], "first@example.com")
	}
	if got[secondID] != "second@example.com" {
		t.Errorf("creator for %s = %q, want %q", secondID, got[secondID], "second@example.com")
	}
	if len(got) != 2 {
		t.Errorf("AssignmentCreators() returned %d creators, want 2", len(got))
	}
}
