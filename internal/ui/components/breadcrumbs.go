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
		isLast := i == len(crumbs)-1
		switch {
		case len(crumb.Alternatives) > 0:
			children = append(children, breadcrumbWithDropdown(crumb, isLast))
		case isLast:
			children = append(children, h.Span(h.Class("active"), g.Text(crumb.Label)))
		case crumb.URL != "":
			children = append(children, h.A(h.Href(crumb.URL), g.Text(crumb.Label)))
		default:
			children = append(children, h.Span(g.Text(crumb.Label)))
		}

		if !isLast {
			children = append(children, g.Text(" / "))
		}
	}

	return h.Nav(h.Class("breadcrumbs"), g.Group(children))
}

func breadcrumbWithDropdown(crumb breadcrumb.Crumb, isActive bool) g.Node {
	items := make([]g.Node, 0, len(crumb.Alternatives))
	for _, alt := range crumb.Alternatives {
		items = append(items, h.A(h.Href(alt.URL), g.Text(alt.Label)))
	}

	var label g.Node
	if isActive {
		label = h.Span(h.Class("active"), g.Text(crumb.Label+" ▾"))
	} else {
		label = h.A(h.Href(crumb.URL), g.Text(crumb.Label+" ▾"))
	}

	return h.Span(h.Class("breadcrumb-switcher"),
		label,
		h.Div(h.Class("breadcrumb-dropdown"), g.Group(items)),
	)
}
