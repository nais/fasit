package database

import (
	"database/sql"
	"time"
)

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
