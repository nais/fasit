package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// KebabButton renders the ⋮ toggle button for a kebab menu.
func KebabButton(targetID string) g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("kebab-btn"),
		g.Attr("data-kebab-toggle", targetID),
		g.Attr("aria-label", "Actions"),
		g.Text("\u22ee"),
	)
}

// KebabMenu renders a kebab dropdown container with the given items.
func KebabMenu(id string, items ...g.Node) g.Node {
	return h.Div(h.Class("kebab-menu"), h.ID(id), g.Group(items))
}

// KebabWrap renders a full kebab: button + menu + any extra nodes (popovers).
func KebabWrap(id string, items []g.Node, extra ...g.Node) g.Node {
	children := []g.Node{
		KebabButton(id),
		KebabMenu(id, items...),
	}
	children = append(children, extra...)
	return h.Div(h.Class("kebab-wrap"), g.Group(children))
}

// Kebab menu item icons.
const (
	IconLoki     = `<svg width="14" height="14" viewBox="0 0 48 56" fill="currentColor"><path d="M12.05 54.92l-.66-4.46-4.46.67.76 4.46 4.36-.67z"/><path d="M46.96 42.4l-.76-4.36-19.45 3.04.57 4.36 19.64-3.04z"/><path d="M20.4 46.58l4.45-.76-.66-4.36-4.46.66.67 4.46z"/><path d="M19.07 53.79l-.76-4.36-4.36.66.57 4.46 4.55-.76z"/><path d="M5.88 44.2l.67 4.46 4.45-.67-.66-4.46-4.46.67z"/><path d="M27.7 47.9l.76 4.56 19.54-3.04-.66-4.46L27.7 47.9z"/><path d="M21.53 53.4l4.36-.57-.66-4.55-4.46.76.76 4.36z"/><path d="M12.81 43.16l.76 4.46 4.36-.67-.66-4.46-4.46.67z"/><path d="M7.4 41.45L1.99 5.98 0 6.26l5.5 35.48 1.9-.29z"/><path d="M9.96 41.07L4.08 2.94l-1.9.38 5.88 38.04 1.9-.29z"/><path d="M14.32 40.41L8.16 0 6.26.38l6.17 40.22 1.89-.19z"/><path d="M16.89 40.03L11.19 3.23l-1.8.28 5.69 36.71 1.81-.19z"/><path d="M21.25 39.27L16.22 6.64l-1.9.28 5.03 32.73 1.9-.38z"/><path d="M23.81 38.89L18.59 5.03l-1.9.28 5.31 33.87 1.81-.29z"/></svg> `
	IconRedeploy = `<svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M13.5 2v4h-4l1.3-1.3A5.5 5.5 0 003 8h-1a6.5 6.5 0 0110.8-4.3L13.5 2zM3 14v-4h4l-1.3 1.3A5.5 5.5 0 0013.5 8h1a6.5 6.5 0 01-10.8 4.3L3 14z"/></svg> `
	IconPause    = `<svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><rect x="2" y="2" width="4" height="12" rx="1"/><rect x="10" y="2" width="4" height="12" rx="1"/></svg> `
	IconPlay     = `<svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M3 2l11 6-11 6V2z"/></svg> `
	IconDocument = `<svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M2 1h8l4 4v10H2V1zm1 1v12h10V5.5L7.5 2H3zm2 5h6v1H5V7zm0 2h6v1H5V9zm0 2h4v1H5v-1z"/></svg> `
	IconLogs     = `<svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor"><path d="M2 2h12v12H2V2zm1 1v10h10V3H3zm1 2h8v1H4V5zm0 2h8v1H4V7zm0 2h5v1H4V9z"/></svg> `
	IconHistory  = `<svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="8" cy="8" r="6.25"/><path d="M8 4.5V8l2.5 1.5"/></svg> `
)

// LokiLogsItem renders a kebab menu item linking to Loki.
func LokiLogsItem(lokiURL string) g.Node {
	return h.A(h.Href(lokiURL), h.Class("kebab-item"), g.Attr("target", "_blank"), g.Attr("rel", "noopener"),
		g.Raw(IconLoki),
		g.Text("Loki logs ↗"),
	)
}
