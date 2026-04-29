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
			navItem("/ui", "Tenants", "tenants"),
			navItem("/ui/features", "Features", "features"),
			navItem("/ui/rollouts", "Rollouts", "rollouts"),
			navItem("/ui/labels", "Labels", "labels"),
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
