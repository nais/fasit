package view

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type TenantNav struct {
	Name string
}

type EnvironmentNav struct {
	Name string
}

type FeatureNav struct {
	Name         string
	Enabled      bool
	FailedCount  int
	PendingCount int
}

func TextOrMuted(value, fallback string) g.Node {
	if value == "" {
		return h.Span(h.Class("text-muted"), g.Text(fallback))
	}
	return g.Text(value)
}
