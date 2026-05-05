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
	switch {
	case feature.FailedCount > 0:
		return h.Span(
			h.Class("feature-nav-badge status-error"),
			h.Title(strconv.Itoa(feature.FailedCount)+" failed deployment(s)"),
			g.Text("✗ "+strconv.Itoa(feature.FailedCount)),
		)
	case feature.PendingCount > 0:
		return h.Span(
			h.Class("feature-nav-badge status-pending"),
			h.Title(strconv.Itoa(feature.PendingCount)+" pending deployment(s)"),
			g.Text("⏳ "+strconv.Itoa(feature.PendingCount)),
		)
	}
	return nil
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
				if inEnv && syncEnabled {
					status = h.Span(h.Style("color: green"), g.Text("✓"))
				} else if inEnv {
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
