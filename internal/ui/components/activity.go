package components

import (
	"encoding/json"
	"strings"

	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ActivityListParams configures the activity list component.
type ActivityListParams struct {
	Title       string
	FilterBadge string // optional badge text (e.g. "tenant/env")
	AllHref     string // link for "All →"
	Entries     []*audit.Entry
	// ResourceNode renders the resource label per entry.
	ResourceNode func(*audit.Entry) g.Node
	// DescriptionFn returns a text description for an entry. If nil, uses Entry.Summary().
	DescriptionFn func(*audit.Entry) string
}

// ActivityList renders a compact activity sidebar section.
func ActivityList(p ActivityListParams) g.Node {
	if len(p.Entries) == 0 {
		return nil
	}

	resourceFn := p.ResourceNode
	if resourceFn == nil {
		resourceFn = func(e *audit.Entry) g.Node { return g.Text(e.ObjectID) }
	}
	descFn := p.DescriptionFn
	if descFn == nil {
		descFn = func(e *audit.Entry) string { return e.Summary() }
	}

	items := make([]g.Node, 0, len(p.Entries))
	for _, e := range p.Entries {
		description := descFn(e)
		items = append(items, h.Li(
			h.Div(h.Class("activity-meta"),
				h.Span(
					g.Text(string(e.Action)),
					g.If(e.Actor != "", g.Group([]g.Node{g.Text(" by "), h.Span(h.Class("activity-actor"), view.ActorNode(e.Actor))})),
				),
				h.Span(h.Title(view.FormatTime(e.CreatedAt)), g.Text(view.RelativeTime(e.CreatedAt))),
			),
			h.Div(h.Class("activity-resource"), resourceFn(e)),
			ConfigChangeNode(e),
			g.If(description != "", h.Div(h.Class("activity-description"), g.Text(description))),
		))
	}

	headerContent := []g.Node{
		h.Div(
			h.H3(g.Text(p.Title)),
			g.If(p.FilterBadge != "", h.Span(h.Class("activity-filter-badge"), g.Text(p.FilterBadge))),
		),
	}
	if p.AllHref != "" {
		headerContent = append(headerContent, h.A(h.Href(p.AllHref), h.Class("link-muted"), g.Text("All →")))
	}

	return h.Section(h.Class("activity-section"),
		h.Div(h.Class("activity-header"), g.Group(headerContent)),
		h.Ul(h.Class("activity-list"), g.Group(items)),
	)
}

// ConfigChangeNode renders a compact old→new diff for config audit entries.
func ConfigChangeNode(e *audit.Entry) g.Node {
	if e.ObjectType != audit.ObjectTypeConfiguration {
		return nil
	}
	if len(e.Metadata) == 0 {
		return nil
	}
	var meta map[string]string
	if json.Unmarshal(e.Metadata, &meta) != nil {
		return nil
	}
	if meta["secret"] == "true" {
		return h.Span(h.Class("text-muted"), g.Text("(secret)"))
	}
	oldVal, hasOld := meta["old"]
	newVal, hasNew := meta["new"]
	if !hasOld && !hasNew {
		return nil
	}
	oldClean := cleanVal(oldVal)
	newClean := cleanVal(newVal)
	if !hasOld {
		return h.Code(h.Title(newClean), g.Text(truncateVal(newClean)))
	}
	if !hasNew {
		return h.Code(h.Class("val-deleted"), h.Title(oldClean), g.Text(truncateVal(oldClean)))
	}
	return h.Span(h.Class("config-change"), h.Title(oldClean+" → "+newClean),
		h.Code(h.Class("val-old"), g.Text(truncateVal(oldClean))),
		g.Text(" → "),
		h.Code(g.Text(truncateVal(newClean))),
	)
}

func cleanVal(s string) string {
	s = strings.TrimPrefix(s, `"`)
	s = strings.TrimSuffix(s, `"`)
	return s
}

func truncateVal(s string) string {
	const max = 24
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
