// Package auditfake provides a test fake for auditsql.Querier.
package auditfake

import (
	"context"

	"github.com/nais/fasit/internal/audit/auditsql"
)

// Querier is a test fake that records calls. Assign function fields to
// control return values; unset fields panic on use.
type Querier struct {
	AuditCreateFunc         func(ctx context.Context, arg auditsql.AuditCreateParams) error
	AuditForEnvironmentFunc func(ctx context.Context, arg auditsql.AuditForEnvironmentParams) ([]auditsql.Audit, error)

	// Creates records every AuditCreate call for assertion.
	Creates []auditsql.AuditCreateParams
}

func (f *Querier) AuditCreate(ctx context.Context, arg auditsql.AuditCreateParams) error {
	f.Creates = append(f.Creates, arg)
	if f.AuditCreateFunc != nil {
		return f.AuditCreateFunc(ctx, arg)
	}
	return nil
}

func (f *Querier) AuditForEnvironment(ctx context.Context, arg auditsql.AuditForEnvironmentParams) ([]auditsql.Audit, error) {
	if f.AuditForEnvironmentFunc == nil {
		panic("auditfake: AuditForEnvironmentFunc not set")
	}
	return f.AuditForEnvironmentFunc(ctx, arg)
}

var _ auditsql.Querier = (*Querier)(nil)
