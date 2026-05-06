package view

import (
	"strconv"
	"time"
)

var Oslo = mustLoadLocation("Europe/Oslo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func FormatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(Oslo).Format("2006-01-02 15:04:05")
}

func FormatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return FormatTime(*t)
}

func RelativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		return FormatTime(t)
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return formatUnit(int(d/time.Minute), "m")
	case d < 24*time.Hour:
		return formatUnit(int(d/time.Hour), "h")
	case d < 48*time.Hour:
		return "yesterday"
	case d < 30*24*time.Hour:
		return formatUnit(int(d/(24*time.Hour)), "d")
	case d < 365*24*time.Hour:
		return formatUnit(int(d/(30*24*time.Hour)), "mo")
	default:
		return formatUnit(int(d/(365*24*time.Hour)), "y")
	}
}

func formatUnit(n int, unit string) string {
	return strconv.Itoa(n) + unit + " ago"
}
