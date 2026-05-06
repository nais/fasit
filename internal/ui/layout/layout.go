package layout

import (
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

type Props struct {
	Title        string
	CurrentPage  components.Page
	Content      g.Node
	UserEmail    string
	AssetVersion string
}

func Page(props Props) g.Node {
	title := "Fasit"
	if props.Title != "" {
		title = "Fasit - " + props.Title
	}

	v := ""
	if props.AssetVersion != "" {
		v = "?v=" + props.AssetVersion
	}

	return c.HTML5(c.HTML5Props{
		Title:    title,
		Language: "en",
		Head: []g.Node{
			h.Meta(h.Name("viewport"), h.Content("width=1024")),
			h.Meta(h.Name("color-scheme"), h.Content("dark light")),
			h.Link(h.Rel("icon"), h.Href("/favicon.ico"+v)),
			h.Script(g.Raw(`(function(){var t=localStorage.getItem("theme");if(t)document.documentElement.dataset.theme=t})()`)),
			h.StyleEl(g.Raw(`html{background:#1a1a1a;color:#ddd}html[data-theme="light"]{background:#f5f5f5;color:#333}`)),
			h.Link(h.Rel("stylesheet"), h.Href("/site.css"+v)),
			h.Script(h.Src("/site.js"+v), h.Defer()),
		},
		Body: []g.Node{
			components.SiteHeader(props.CurrentPage, props.UserEmail),
			props.Content,
		},
	})
}
