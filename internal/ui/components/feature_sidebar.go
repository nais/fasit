package components

import (
	"github.com/nais/fasit/internal/ui/featureenvs"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeatureSidebar(featureName, activeTab, activeTenant, activeEnvironment string, envs []featureenvs.Environment) g.Node {
	return h.Aside(
		h.Class("sidebar feature-sidebar"),
		h.Div(
			h.Class("feature-sidebar-header"),
			h.H4(g.Text(featureName)),
		),
		h.Div(
			h.Class("nav"),
			h.Ul(
				featureNavItem("/features/"+featureName, "Overview", activeTab == "overview"),
				featureNavItem("/features/"+featureName+"/versions", "Versions", activeTab == "versions"),
				featureNavItem("/features/"+featureName+"/assignments", "Assignments", activeTab == "assignments"),
				featureNavItem("/features/"+featureName+"/config", "Config", activeTab == "config"),
			),
			h.Div(h.Class("sidebar-section-label"), g.Text("Environments")),
			h.Ul(g.Group(g.Map(envs, func(env featureenvs.Environment) g.Node {
				isActive := env.TenantSlug == activeTenant && env.EnvironmentName == activeEnvironment
				return featureEnvironmentItem(featureName, env, isActive)
			}))),
		),
	)
}

func featureNavItem(href, label string, active bool) g.Node {
	attrs := []g.Node{h.Href(href)}
	if active {
		attrs = append(attrs, h.Class("active"))
	}
	return h.Li(h.A(append(attrs, g.Text(label))...))
}

func featureEnvironmentItem(featureName string, env featureenvs.Environment, active bool) g.Node {
	className := "feature-env-link"
	if active {
		className += " active"
	}
	return h.Li(h.A(
		h.Href("/features/"+featureName+"/envs/"+env.TenantSlug+"/"+env.EnvironmentName),
		h.Class(className),
		h.Span(h.Class("feature-env-dot "+StatusClass(env.Status)), h.Title(env.Status)),
		h.Span(g.Text(env.TenantName+" / "+env.EnvironmentName)),
	))
}
