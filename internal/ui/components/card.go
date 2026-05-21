package components

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func Card(children ...g.Node) g.Node {
	return h.Div(h.Class("card"), h.Div(h.Class("card-body"), g.Group(children)))
}

func CardCompact(children ...g.Node) g.Node {
	return h.Div(h.Class("card"), h.Div(h.Class("card-body card-compact"), g.Group(children)))
}
