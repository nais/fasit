package view

import "strings"

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

func TenantIcon(name string) string {
	icons := map[string]string{
		"nav":      "N",
		"devnais":  "DN",
		"testnais": "TN",
		"cinais":   "CI",
		"atil":     "AT",
		"ssb":      "SSB",
		"ldir":     "LD",
	}
	if icon, ok := icons[name]; ok {
		return icon
	}
	if len(name) >= 2 {
		return strings.ToUpper(name[:2])
	}
	return strings.ToUpper(name)
}

func TenantColor(name string) string {
	colors := map[string]string{
		"nav":      "#3B82F6",
		"devnais":  "#8B5CF6",
		"testnais": "#F59E0B",
		"cinais":   "#EF4444",
		"atil":     "#10B981",
		"ssb":      "#06B6D4",
		"ldir":     "#22C55E",
	}
	if color, ok := colors[name]; ok {
		return color
	}
	return "#6B7280"
}
