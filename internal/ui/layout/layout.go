package layout

import (
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

type Props struct {
	Title       string
	CurrentPage components.Page
	Content     g.Node
}

func Page(props Props) g.Node {
	title := "Fasit"
	if props.Title != "" {
		title = "Fasit - " + props.Title
	}

	return c.HTML5(c.HTML5Props{
		Title:    title,
		Language: "en",
		Head: []g.Node{
			h.Meta(h.Name("viewport"), h.Content("width=1024")),
			h.Script(g.Raw(`(function(){var t=localStorage.getItem("theme");if(t)document.documentElement.dataset.theme=t})()`)),
			h.Link(h.Rel("stylesheet"), h.Href("/ui/site.css")),
			h.Script(h.Src("/ui/site.js"), h.Defer()),
		},
		Body: []g.Node{
			components.SiteHeader(props.CurrentPage),
			props.Content,
		},
	})
}
