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
	PageLabels      Page = "labels"
	PageNaisd       Page = "naisd"
)

func SiteHeader(currentPage Page, userEmail string, gcpProjectID string) g.Node {
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
		g.If(gcpProjectID != "",
			h.A(
				h.Href("https://console.cloud.google.com/welcome?project="+gcpProjectID+"&authuser="+userEmail),
				h.Class("item"),
				g.Attr("target", "_blank"),
				g.Attr("rel", "noopener noreferrer"),
				g.Attr("title", "Open GCP project "+gcpProjectID),
				g.Text("Open GCP project"),
			),
		),
		h.Button(
			h.Type("button"),
			h.Class("nav-btn"),
			g.Attr("title", "Trigger a full reconcile of all features"),
			g.Attr("popovertarget", "reconcile-confirm"),
			g.Text("Reconcile all features"),
		),
		h.Div(g.Attr("popover", ""), h.ID("reconcile-confirm"),
			h.H3(g.Text("Confirm reconcile")),
			h.P(g.Text("Trigger a full reconcile of all deployments?")),
			h.Form(h.Method("post"), h.Action("/reconcile"),
				h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text("Reconcile")), h.Button(h.Type("button"), g.Attr("popovertarget", "reconcile-confirm"), g.Attr("popovertargetaction", "hide"), g.Text("Cancel"))),
			),
		),
		h.Button(h.Class("theme-toggle"), g.Attr("onclick", "toggleTheme()"), g.Attr("title", "Toggle light/dark mode"), g.Raw("☀︎")),
		h.Span(h.Class("user"), g.Text(userEmail)),
	)
}
