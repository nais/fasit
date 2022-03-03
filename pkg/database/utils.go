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

func ptrToNullTime(time *time.Time) sql.NullTime {
	if time == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{
		Time:  *time,
		Valid: true,
	}
}

func nullStringToPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}

	return &ns.String
}

func nullTimeToPtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}

	return &nt.Time
}

func ptrToString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func stringToPtr(s string) *string {
	return &s
}
