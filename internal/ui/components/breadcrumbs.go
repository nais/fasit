package components

import (
	"github.com/nais/fasit/internal/ui/breadcrumb"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func Breadcrumbs(crumbs []breadcrumb.Crumb, actions ...g.Node) g.Node {
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
			var label []g.Node
			if crumb.Icon != nil {
				label = append(label, crumb.Icon)
			}
			if crumb.Content != nil {
				label = append(label, crumb.Content)
			} else {
				label = append(label, g.Text(crumb.Label))
			}
			if crumb.Subtitle != "" {
				label = append(label, h.Span(h.Class("breadcrumb-subtitle"), g.Text("("+crumb.Subtitle+")")))
			}
			children = append(children, h.Span(h.Class("active"), g.Group(label)))
		case crumb.URL != "":
			var nodes []g.Node
			if crumb.Icon != nil {
				nodes = append(nodes, crumb.Icon)
			}
			nodes = append(nodes, g.Text(crumb.Label))
			children = append(children, h.A(h.Href(crumb.URL), g.Group(nodes)))
		default:
			var nodes []g.Node
			if crumb.Icon != nil {
				nodes = append(nodes, crumb.Icon)
			}
			nodes = append(nodes, g.Text(crumb.Label))
			children = append(children, h.Span(g.Group(nodes)))
		}

		if !isLast {
			children = append(children, h.Span(h.Class("breadcrumb-sep"), g.Raw("&#x203A;")))
		}
	}

	// Source link from the last crumb, pushed to the right.
	lastCrumb := crumbs[len(crumbs)-1]
	if lastCrumb.SourceURL != "" {
		children = append(children, h.A(
			h.Class("breadcrumb-source"),
			h.Href(lastCrumb.SourceURL),
			h.Target("_blank"),
			g.Attr("title", "Source"),
			g.Raw(`<svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><path fill-rule="evenodd" d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>`),
		))
	}

	// Extra actions pushed to the right, after source link.
	children = append(children, actions...)

	return h.Nav(h.Class("breadcrumbs"), g.Group(children))
}

func breadcrumbWithDropdown(crumb breadcrumb.Crumb, isActive bool) g.Node {
	items := make([]g.Node, 0, len(crumb.Alternatives))
	for _, alt := range crumb.Alternatives {
		items = append(items, h.A(h.Href(alt.URL), g.Text(alt.Label)))
	}

	var labelNodes []g.Node
	if crumb.Icon != nil {
		labelNodes = append(labelNodes, crumb.Icon)
	}
	labelNodes = append(labelNodes, g.Text(crumb.Label+" ▾"))

	var label g.Node
	if isActive {
		label = h.Span(h.Class("active"), g.Group(labelNodes))
	} else {
		label = h.A(h.Href(crumb.URL), g.Group(labelNodes))
	}

	return h.Span(h.Class("breadcrumb-switcher"),
		label,
		h.Div(h.Class("breadcrumb-dropdown"), g.Group(items)),
	)
}
