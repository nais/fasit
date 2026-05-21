// Package auditsqlfake provides a test fake for auditsql.Querier.
package auditsqlfake

import (
	"context"

	"github.com/nais/fasit/internal/audit/auditsql"
)

var _ auditsql.Querier = (*Querier)(nil)

// Querier is a test fake that records calls. Assign function fields to
// control return values; unset fields return nil.
type Querier struct {
	AuditCreateFunc func(ctx context.Context, arg auditsql.CreateParams) error

	// Creates records every Create call for assertion.
	Creates []auditsql.CreateParams
}

func (f *Querier) Create(ctx context.Context, arg auditsql.CreateParams) error {
	f.Creates = append(f.Creates, arg)
	if f.AuditCreateFunc != nil {
		return f.AuditCreateFunc(ctx, arg)
	}
	return nil
}

func (f *Querier) List(ctx context.Context, arg auditsql.ListParams) ([]auditsql.ListRow, error) {
	return nil, nil
}

func (f *Querier) ListForEnvironment(_ context.Context, _ auditsql.ListForEnvironmentParams) ([]auditsql.ListForEnvironmentRow, error) {
	return nil, nil
}

func (f *Querier) ListRecent(_ context.Context, _ int32) ([]auditsql.ListRecentRow, error) {
	return nil, nil
}
