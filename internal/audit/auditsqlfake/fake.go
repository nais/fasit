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
	AuditCreateFunc       func(ctx context.Context, arg auditsql.CreateParams) error
	AuditSearchRecentFunc func(ctx context.Context, arg auditsql.SearchRecentParams) ([]auditsql.SearchRecentRow, error)

	// Creates records every Create call for assertion.
	Creates []auditsql.CreateParams

	// SearchRecentCalls records every SearchRecent call for assertion.
	SearchRecentCalls []auditsql.SearchRecentParams
}

func (f *Querier) Create(ctx context.Context, arg auditsql.CreateParams) error {
	f.Creates = append(f.Creates, arg)
	if f.AuditCreateFunc != nil {
		return f.AuditCreateFunc(ctx, arg)
	}
	return nil
}

func (f *Querier) ListForFeature(_ context.Context, _ auditsql.ListForFeatureParams) ([]auditsql.ListForFeatureRow, error) {
	return nil, nil
}

func (f *Querier) ListAssignmentsForFeature(_ context.Context, _ auditsql.ListAssignmentsForFeatureParams) ([]auditsql.ListAssignmentsForFeatureRow, error) {
	return nil, nil
}

func (f *Querier) ListForFeatureInEnvironment(_ context.Context, _ auditsql.ListForFeatureInEnvironmentParams) ([]auditsql.ListForFeatureInEnvironmentRow, error) {
	return nil, nil
}

func (f *Querier) ListConfigForFeatureInEnvironment(_ context.Context, _ auditsql.ListConfigForFeatureInEnvironmentParams) ([]auditsql.ListConfigForFeatureInEnvironmentRow, error) {
	return nil, nil
}

func (f *Querier) ListForEnvironment(_ context.Context, _ auditsql.ListForEnvironmentParams) ([]auditsql.ListForEnvironmentRow, error) {
	return nil, nil
}

func (f *Querier) ListRecent(_ context.Context, _ int32) ([]auditsql.ListRecentRow, error) {
	return nil, nil
}

func (f *Querier) SearchRecent(ctx context.Context, arg auditsql.SearchRecentParams) ([]auditsql.SearchRecentRow, error) {
	f.SearchRecentCalls = append(f.SearchRecentCalls, arg)
	if f.AuditSearchRecentFunc != nil {
		return f.AuditSearchRecentFunc(ctx, arg)
	}
	return nil, nil
}

func (f *Querier) LatestDisableReason(_ context.Context, _ auditsql.LatestDisableReasonParams) (string, error) {
	return "", nil
}
