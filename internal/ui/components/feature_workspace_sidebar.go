package components

import (
	"github.com/nais/fasit/internal/ui/featureworkspace"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeatureWorkspaceSidebar(featureName, activeTab, activeTenant, activeEnvironment string, envs []featureworkspace.Environment) g.Node {
	return h.Aside(h.Class("sidebar feature-workspace-sidebar"),
		h.Div(h.Class("feature-workspace-header"),
			h.H4(g.Text(featureName)),
		),
		h.Div(h.Class("nav"),
			h.Ul(
				workspaceNavItem("/features/"+featureName, "Overview", activeTab == "overview"),
				workspaceNavItem("/features/"+featureName+"/deploy-specs", "Deploy specs", activeTab == "deploy-specs"),
				workspaceNavItem("/features/"+featureName+"/config", "Config", activeTab == "config"),
				workspaceNavItem("/features/"+featureName+"/config-explorer", "Config explorer", activeTab == "config-explorer"),
			),
			h.Div(h.Class("sidebar-section-label"), g.Text("Environments")),
			h.Ul(g.Group(g.Map(envs, func(env featureworkspace.Environment) g.Node {
				isActive := env.TenantSlug == activeTenant && env.EnvironmentName == activeEnvironment
				return workspaceEnvironmentItem(featureName, env, isActive)
			}))),
		),
	)
}

func workspaceNavItem(href, label string, active bool) g.Node {
	attrs := []g.Node{h.Href(href)}
	if active {
		attrs = append(attrs, h.Class("active"))
	}
	return h.Li(h.A(append(attrs, g.Text(label))...))
}

func workspaceEnvironmentItem(featureName string, env featureworkspace.Environment, active bool) g.Node {
	className := "workspace-env-link"
	if active {
		className += " active"
	}
	return h.Li(h.A(
		h.Href("/features/"+featureName+"/envs/"+env.TenantSlug+"/"+env.EnvironmentName),
		h.Class(className),
		h.Span(h.Class("workspace-env-dot "+StatusClass(env.Status)), h.Title(env.Status)),
		h.Span(g.Text(env.TenantName+" / "+env.EnvironmentName)),
	))
}
