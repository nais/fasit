package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type Tab struct {
	ID    string
	Href  string
	Label string
}

func TabsNav(activeTab string, tabs []Tab) g.Node {
	return h.Nav(h.Class("tabs-nav"),
		g.Group(g.Map(tabs, func(tab Tab) g.Node {
			attrs := []g.Node{h.Href(tab.Href)}
			if activeTab == tab.ID {
				attrs = append(attrs, h.Class("active"))
			}

			return h.A(append(attrs, g.Text(tab.Label))...)
		})),
	)
}
