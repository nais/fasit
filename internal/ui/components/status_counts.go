package components

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func StatusCountsBadge(failed, pending int) g.Node {
	var badges []g.Node
	if failed > 0 {
		badges = append(badges, h.Span(
			h.Class("status-badge status-error"),
			h.Title(strconv.Itoa(failed)+" failed"),
			g.Text(strconv.Itoa(failed)+" failed"),
		))
	}
	if pending > 0 {
		badges = append(badges, h.Span(
			h.Class("status-badge status-pending"),
			h.Title(strconv.Itoa(pending)+" pending"),
			g.Text(strconv.Itoa(pending)+" pending"),
		))
	}
	if len(badges) == 0 {
		return nil
	}
	return g.Group(badges)
}
