// Package auditfake provides a test fake for auditsql.Querier.
package auditfake

import (
	"context"

	"github.com/nais/fasit/internal/audit/auditsql"
)

// Querier is a test fake that records calls. Assign function fields to
// control return values; unset fields panic on use.
type Querier struct {
	AuditCreateFunc         func(ctx context.Context, arg auditsql.CreateParams) error
	AuditForEnvironmentFunc func(ctx context.Context, arg auditsql.ListParams) ([]auditsql.Audit, error)

	// Creates records every AuditCreate call for assertion.
	Creates []auditsql.CreateParams
}

func (f *Querier) Create(ctx context.Context, arg auditsql.CreateParams) error {
	f.Creates = append(f.Creates, arg)
	if f.AuditCreateFunc != nil {
		return f.AuditCreateFunc(ctx, arg)
	}
	return nil
}

func (f *Querier) List(ctx context.Context, arg auditsql.ListParams) ([]auditsql.Audit, error) {
	if f.AuditForEnvironmentFunc == nil {
		panic("auditfake: AuditForEnvironmentFunc not set")
	}
	return f.AuditForEnvironmentFunc(ctx, arg)
}

var _ auditsql.Querier = (*Querier)(nil)
