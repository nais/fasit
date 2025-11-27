package database

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// ToInt32 safely converts an int to int32, returning an error if the value is out of bounds.
func ToInt32(val int) (int32, error) {
	if val > math.MaxInt32 || val < math.MinInt32 {
		return 0, fmt.Errorf("ToInt32: value %d out of int32 bounds (min: %d, max: %d)", val, math.MinInt32, math.MaxInt32)
	}
	return int32(val), nil
}

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
