package components

import (
	"strconv"

	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeaturesSidebar(features []view.FeatureNav, currentFeatureName string) g.Node {
	return h.Aside(h.Class("sidebar"),
		h.Div(h.Class("nav"),
			h.Ul(g.Group(g.Map(features, func(feature view.FeatureNav) g.Node {
				attrs := []g.Node{h.Href("/features/" + feature.Name)}
				if currentFeatureName != "" && feature.Name == currentFeatureName {
					attrs = append(attrs, h.Class("active"))
				}

				return h.Li(
					h.A(append(attrs, g.Text(feature.Name), featureStatusBadge(feature))...),
				)
			}))),
		),
	)
}

func featureStatusBadge(feature view.FeatureNav) g.Node {
	return StatusCountsBadge(feature.FailedCount, feature.PendingCount)
}

// StatusCountsBadge renders small ✗N and/or ⏳N badges. When both failed and
// pending are non-zero, both badges are shown (failed first). Returns nil when
// both are zero so callers can append it unconditionally.
func StatusCountsBadge(failed, pending int) g.Node {
	var badges []g.Node
	if failed > 0 {
		badges = append(badges, h.Span(
			h.Class("feature-nav-badge status-error"),
			h.Title(strconv.Itoa(failed)+" failed"),
			g.Text("✗ "+strconv.Itoa(failed)),
		))
	}
	if pending > 0 {
		badges = append(badges, h.Span(
			h.Class("feature-nav-badge status-pending"),
			h.Title(strconv.Itoa(pending)+" pending"),
			g.Text("⏳ "+strconv.Itoa(pending)),
		))
	}
	if len(badges) == 0 {
		return nil
	}
	return g.Group(badges)
}

func EnvironmentSidebar(tenantName, environmentName, currentFeatureName string, allFeatures, enabledFeatures []view.FeatureNav) g.Node {
	enabledMap := make(map[string]bool, len(enabledFeatures))
	for _, feature := range enabledFeatures {
		enabledMap[feature.Name] = feature.Enabled
	}

	return h.Aside(h.Class("sidebar"),
		h.Div(h.Class("nav"),
			h.Ul(g.Group(g.Map(allFeatures, func(feature view.FeatureNav) g.Node {
				syncEnabled, inEnv := enabledMap[feature.Name]
				attrs := []g.Node{
					h.Href("/tenants/" + tenantName + "/envs/" + environmentName + "/" + feature.Name),
				}
				if currentFeatureName != "" && feature.Name == currentFeatureName {
					attrs = append(attrs, h.Class("active"))
				}
				if !inEnv {
					attrs = append(attrs, h.Style("opacity: 0.5;"))
				}

				status := h.Span(h.Style("color: gray"), g.Text("○"))
				switch {
				case feature.FailedCount > 0:
					status = h.Span(h.Class("status-error"), h.Title("failed"), g.Text("✗"))
				case feature.PendingCount > 0:
					status = h.Span(h.Class("status-pending"), h.Title("pending"), g.Text("⏳"))
				case inEnv && syncEnabled:
					status = h.Span(h.Style("color: green"), g.Text("✓"))
				case inEnv:
					status = h.Span(h.Style("color: orange"), g.Text("⏸"))
				}

				return h.Li(
					h.A(append(attrs,
						status,
						g.Text(" "),
						g.Text(feature.Name),
					)...),
				)
			}))),
		),
	)
}
