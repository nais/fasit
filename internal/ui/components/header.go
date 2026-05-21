package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type Page string

const (
	PageEnvironments Page = "environments"
	PageFeatures     Page = "features"
	PageDeployments  Page = "deployments"
	PageLabels       Page = "labels"
	PageNaisd        Page = "naisd"
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
			navItem("/features", "Features", PageFeatures),
			navItem("/environments", "Environments", PageEnvironments),
			navItem("/deployments", "Deployments", PageDeployments),
			navItem("/labels", "Labels", PageLabels),
			navItem("/naisd", "Naisd", PageNaisd),
			h.A(
				h.Href("https://vedtak.nais.io/"),
				h.Class("item"),
				g.Attr("target", "_blank"),
				g.Attr("rel", "noopener noreferrer"),
				g.Text("Cluster management"),
				externalLinkIcon(),
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

func externalLinkIcon() g.Node {
	return g.Raw(`<svg class="external-link-icon" width="14" height="14" viewBox="0 0 24 24" fill="none" xmlns="http://www.w3.org/2000/svg"><path fill-rule="evenodd" clip-rule="evenodd" d="M20.5319 3.47126C20.603 3.5428 20.6568 3.62511 20.6931 3.71291C20.7298 3.80134 20.75 3.89831 20.75 4V11.5C20.75 11.9142 20.4142 12.25 20 12.25C19.5858 12.25 19.25 11.9142 19.25 11.5V5.81066L10.5303 14.5303C10.2374 14.8232 9.76256 14.8232 9.46967 14.5303C9.17678 14.2374 9.17678 13.7626 9.46967 13.4697L18.1893 4.75H12.5C12.0858 4.75 11.75 4.41421 11.75 4C11.75 3.58579 12.0858 3.25 12.5 3.25H20C20.2063 3.25 20.3931 3.33329 20.5287 3.46808L20.5303 3.46967L20.5319 3.47126ZM4.75 9C4.75 8.86193 4.86193 8.75 5 8.75H12C12.4142 8.75 12.75 8.41421 12.75 8C12.75 7.58579 12.4142 7.25 12 7.25H5C4.0335 7.25 3.25 8.0335 3.25 9V19C3.25 19.9665 4.0335 20.75 5 20.75H15C15.9665 20.75 16.75 19.9665 16.75 19V12C16.75 11.5858 16.4142 11.25 16 11.25C15.5858 11.25 15.25 11.5858 15.25 12V19C15.25 19.1381 15.1381 19.25 15 19.25H5C4.86193 19.25 4.75 19.1381 4.75 19V9Z" fill="currentColor"/></svg>`)
}
