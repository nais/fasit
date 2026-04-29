package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type Page string

const (
	PageTenants     Page = "tenants"
	PageFeatures    Page = "features"
	PageDeployments Page = "deployments"
	PageRollouts    Page = "rollouts"
	PageLabels      Page = "labels"
)

func SiteHeader(currentPage Page) g.Node {
	navItem := func(href, label string, page Page) g.Node {
		className := "item"
		if currentPage == page {
			className += " active"
		}

		return h.A(
			h.Href(href),
			h.Class(className),
			g.Text(label),
		)
	}

	return h.Nav(
		h.A(h.Href("/ui/"), h.Class("logo"), g.Text("Fasit")),
		h.Div(h.Class("menu"),
			navItem("/ui", "Tenants", PageTenants),
			navItem("/ui/features", "Features", PageFeatures),
			navItem("/ui/deployments", "Deployments", PageDeployments),
			navItem("/ui/rollouts", "Rollouts", PageRollouts),
			navItem("/ui/labels", "Labels", PageLabels),
			h.A(
				h.Href("https://vedtak.nais.io/"),
				h.Class("item"),
				g.Attr("target", "_blank"),
				g.Attr("rel", "noopener noreferrer"),
				g.Text("Cluster management"),
			),
		),
		h.Button(h.Class("theme-toggle"), g.Attr("onclick", "toggleTheme()"), g.Attr("title", "Toggle light/dark mode"), g.Raw("☀︎")),
		h.Span(h.Class("user"), g.Text("user")),
	)
}
