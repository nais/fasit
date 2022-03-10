package database

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
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

func nullUUIDToPtr(uid uuid.NullUUID) *model.ID {
	if uid.Valid {
		mid := model.ID(uid.UUID)
		return &mid
	}
	return nil
}

func ptrToNullUUID(uid *model.ID) uuid.NullUUID {
	if uid == nil {
		return uuid.NullUUID{}
	}
	return uuid.NullUUID{
		UUID:  uuid.UUID(*uid),
		Valid: true,
	}
}
