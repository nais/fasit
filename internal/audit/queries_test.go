package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/audit/auditsql/auditfake"
	"github.com/sirupsen/logrus/hooks/test"
)

func TestCreate(t *testing.T) {
	type meta struct {
		Verb string `json:"verb"`
		Key  string `json:"key"`
	}

	tests := []struct {
		name     string
		params   CreateParams
		wantDesc string
		wantMeta []byte // nil means SQL NULL
	}{
		{
			name: "Create(metadata): marshals to JSON",
			params: CreateParams{
				Description: "desc",
				ObjectType:  "ot",
				ObjectID:    "oid",
				Metadata:    meta{Verb: "v", Key: "k"},
			},
			wantDesc: "desc",
			wantMeta: mustJSON(t, meta{Verb: "v", Key: "k"}),
		},
		{
			name: "Create(nil metadata): stores NULL",
			params: CreateParams{
				Description: "d",
				ObjectType:  "ot",
				ObjectID:    "oid",
			},
			wantDesc: "d",
			wantMeta: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			log, _ := test.NewNullLogger()
			q := &auditfake.Querier{}
			ctx := RegisterTestDeps(context.Background(), q, log)

			if err := Create(ctx, tc.params); err != nil {
				t.Fatal(err)
			}

			if len(q.Creates) != 1 {
				t.Fatalf("got %d calls, want 1", len(q.Creates))
			}
			got := q.Creates[0]
			if got.Description != tc.wantDesc {
				t.Errorf("Description = %q, want %q", got.Description, tc.wantDesc)
			}
			if tc.wantMeta == nil && got.Metadata != nil {
				t.Errorf("Metadata = %s, want nil", got.Metadata)
			}
			if tc.wantMeta != nil && !jsonEqual(got.Metadata, tc.wantMeta) {
				t.Errorf("Metadata = %s, want %s", got.Metadata, tc.wantMeta)
			}
		})
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func jsonEqual(a, b []byte) bool {
	var va, vb any
	if err := json.Unmarshal(a, &va); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &vb); err != nil {
		return false
	}
	aj, _ := json.Marshal(va)
	bj, _ := json.Marshal(vb)
	return string(aj) == string(bj)
}

var _ auditsql.Querier = (*auditfake.Querier)(nil)
