package database

import (
	"context"
	"database/sql"
	"time"
)

func WithNow(ctx context.Context, now func() time.Time) context.Context {
	return context.WithValue(ctx, ctxKey("nowfunc"), now)
}

func Now(ctx context.Context) time.Time {
	if now, ok := ctx.Value(ctxKey("nowfunc")).(func() time.Time); ok {
		return now()
	}
	return time.Now()
}

func ptrToNullString(str *string) sql.NullString {
	if str == nil {
		return sql.NullString{}
	}
	return sql.NullString{
		String: *str,
		Valid:  true,
	}
}

func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}

	return &ns.String
}

func stringToPtr(s string) *string {
	return &s
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}
