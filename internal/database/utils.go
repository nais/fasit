package database

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

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
