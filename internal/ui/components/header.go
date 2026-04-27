package components

import (
	"github.com/nais/fasit/internal/ui"
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
		h.A(h.Href(ui.BasePath+"/"), h.Class("logo"), g.Text("Fasit")),
		h.Div(h.Class("menu"),
			navItem(ui.BasePath+"/tenants", "Tenants", "tenants"),
			navItem(ui.BasePath+"/features", "Features", "features"),
			navItem(ui.BasePath+"/rollouts", "Rollouts", "rollouts"),
		),
		h.Span(h.Class("user"), g.Text("user")),
	)
}
