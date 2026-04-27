package components

import (
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeaturesSidebar(features []view.FeatureNav, currentFeatureName string) g.Node {
	return h.Aside(h.Class("sidebar"),
		h.Div(h.Class("nav"),
			h.Ul(g.Group(g.Map(features, func(feature view.FeatureNav) g.Node {
				attrs := []g.Node{h.Href("/ui/features/" + feature.Name)}
				if currentFeatureName != "" && feature.Name == currentFeatureName {
					attrs = append(attrs, h.Class("active"))
				}

				return h.Li(
					h.A(append(attrs, g.Text(feature.Name))...),
				)
			}))),
		),
	)
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
				classes := ""
				if currentFeatureName != "" && feature.Name == currentFeatureName {
					classes = "active"
				}
				attrs := []g.Node{
					h.Href("/ui/tenants/" + tenantName + "/envs/" + environmentName + "/" + feature.Name),
				}
				if classes != "" {
					attrs = append(attrs, h.Class(classes))
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
