package layout

import (
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	c "maragu.dev/gomponents/components"
	h "maragu.dev/gomponents/html"
)

type Props struct {
	Title          string
	CurrentSection string
	Content        g.Node
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
			h.Link(h.Rel("stylesheet"), h.Href("/site.css")),
			h.Script(h.Src("/site.js"), h.Defer()),
		},
		Body: []g.Node{
			components.SiteHeader(props.CurrentSection),
			props.Content,
		},
	})
}
