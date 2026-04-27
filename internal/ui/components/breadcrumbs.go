package components

import (
	"github.com/nais/fasit/internal/ui/breadcrumb"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func Breadcrumbs(crumbs []breadcrumb.Crumb) g.Node {
	if len(crumbs) == 0 {
		return nil
	}

	children := make([]g.Node, 0, len(crumbs)*2-1)
	for i, crumb := range crumbs {
		if i == len(crumbs)-1 {
			children = append(children, h.Span(h.Class("active"), g.Text(crumb.Label)))
		} else if len(crumb.Alternatives) > 0 {
			children = append(children, breadcrumbWithDropdown(crumb))
		} else if crumb.URL != "" {
			children = append(children, h.A(h.Href(crumb.URL), g.Text(crumb.Label)))
		} else {
			children = append(children, h.Span(g.Text(crumb.Label)))
		}

		if i < len(crumbs)-1 {
			children = append(children, g.Text(" / "))
		}
	}

	return h.Nav(h.Class("breadcrumbs"), g.Group(children))
}

func breadcrumbWithDropdown(crumb breadcrumb.Crumb) g.Node {
	items := make([]g.Node, 0, len(crumb.Alternatives))
	for _, alt := range crumb.Alternatives {
		items = append(items, h.A(h.Href(alt.URL), g.Text(alt.Label)))
	}

	return h.Span(h.Class("breadcrumb-switcher"),
		h.A(h.Href(crumb.URL), g.Text(crumb.Label+" ▾")),
		h.Div(h.Class("breadcrumb-dropdown"), g.Group(items)),
	)
}
