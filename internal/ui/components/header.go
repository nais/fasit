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
		h.A(h.Href("/"), h.Class("logo"), g.Text("Fasit")),
		h.Div(h.Class("menu"),
			navItem("/tenants", "Tenants", "tenants"),
			navItem("/features", "Features", "features"),
			navItem("/rollouts", "Rollouts", "rollouts"),
		),
		h.Span(h.Class("user"), g.Text("user")),
	)
}
