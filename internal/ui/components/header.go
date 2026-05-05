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
	PageNaisd       Page = "naisd"
)

func SiteHeader(currentPage Page, userEmail string) g.Node {
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
		h.A(h.Href("/"), h.Class("logo"), g.Text("Fasit")),
		h.Div(h.Class("menu"),
			navItem("/", "Tenants", PageTenants),
			navItem("/features", "Features", PageFeatures),
			navItem("/deployments", "Deployments", PageDeployments),
			navItem("/rollouts", "Rollouts", PageRollouts),
			navItem("/labels", "Labels", PageLabels),
			navItem("/naisd", "Naisd", PageNaisd),
			h.A(
				h.Href("https://vedtak.nais.io/"),
				h.Class("item"),
				g.Attr("target", "_blank"),
				g.Attr("rel", "noopener noreferrer"),
				g.Text("Cluster management"),
			),
		),
		h.Form(
			h.Class("nav-action"),
			h.Method("post"),
			h.Action("/reconcile"),
			h.Button(
				h.Type("submit"),
				h.Class("nav-btn"),
				g.Attr("title", "Trigger a full reconcile of all deployments"),
				g.Attr("onclick", "return confirm('Trigger a full reconcile of all deployments?')"),
				g.Text("Reconcile"),
			),
		),
		h.Button(h.Class("theme-toggle"), g.Attr("onclick", "toggleTheme()"), g.Attr("title", "Toggle light/dark mode"), g.Raw("☀︎")),
		h.Span(h.Class("user"), g.Text(userEmail)),
	)
}
