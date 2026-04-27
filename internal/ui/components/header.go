package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func SiteHeader(currentSection string) g.Node {
	navItem := func(href, label, section string) g.Node {
		className := "item"
		if currentSection == section {
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
			navItem("/ui/tenants", "Tenants", "tenants"),
			navItem("/ui/features", "Features", "features"),
			navItem("/ui/rollouts", "Rollouts", "rollouts"),
		),
		h.Span(h.Class("user"), g.Text("user")),
	)
}
