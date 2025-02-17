package database

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func Now(ctx context.Context) time.Time {
	if now, ok := ctx.Value(ctxKey("nowfunc")).(func() time.Time); ok {
		return now()
	}
	return time.Now()
}

func ptrToNullString(str *string) pgtype.Text {
	if str == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{
		String: *str,
		Valid:  true,
	}
}

func nullStringToPtr(ns pgtype.Text) *string {
	if !ns.Valid {
		return nil
	}

	return &ns.String
}

func nullTimeToPtr(nt pgtype.Timestamptz) *time.Time {
	if !nt.Valid {
		return nil
	}
	return &nt.Time
}

func nullUUIDToPtr(nu pgtype.UUID) *uuid.UUID {
	if !nu.Valid {
		return nil
	}
	uid := uuid.UUID(nu.Bytes)
	return &uid
}
