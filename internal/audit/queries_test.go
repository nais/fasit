package audit

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nais/fasit/internal/audit/auditsql"
	"github.com/nais/fasit/internal/audit/auditsql/mocks"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/mock"
)

func TestCreate_MarshalsMetadata(t *testing.T) {
	log, _ := test.NewNullLogger()
	q := mocks.NewQuerier(t)
	ctx := RegisterTestDeps(context.Background(), q, log)

	type meta struct {
		Verb string `json:"verb"`
		Key  string `json:"key"`
	}

	q.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(func(p auditsql.AuditCreateParams) bool {
		if p.Description != "desc" || p.ObjectType != "ot" || p.ObjectID != "oid" {
			return false
		}
		var got meta
		if err := json.Unmarshal(p.Metadata, &got); err != nil {
			return false
		}
		return got == meta{Verb: "v", Key: "k"}
	})).Return(nil).Once()

	if err := Create(ctx, CreateParams{
		Description: "desc",
		ObjectType:  "ot",
		ObjectID:    "oid",
		Metadata:    meta{Verb: "v", Key: "k"},
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCreate_NilMetadata(t *testing.T) {
	log, _ := test.NewNullLogger()
	q := mocks.NewQuerier(t)
	ctx := RegisterTestDeps(context.Background(), q, log)

	q.EXPECT().AuditCreate(mock.Anything, mock.MatchedBy(func(p auditsql.AuditCreateParams) bool {
		return p.Metadata == nil
	})).Return(nil).Once()

	if err := Create(ctx, CreateParams{Description: "d", ObjectType: "ot", ObjectID: "oid"}); err != nil {
		t.Fatal(err)
	}
}
