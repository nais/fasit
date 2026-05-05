package rollouts

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Summary struct {
	FeatureName, Version, Status string
	Created                      time.Time
	Completed                    *time.Time
	createdAt                    time.Time
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var items []Summary

		rollouts, err := repo.Rollouts(r.Context(), 50)
		if err == nil {
			for _, rollout := range rollouts {
				items = append(items, Summary{
					FeatureName: rollout.FeatureName,
					Version:     rollout.Version,
					Status:      strings.ToUpper(rollout.Status.String()),
					Created:     rollout.Created,
					Completed:   rollout.Completed,
					createdAt:   rollout.Created,
				})
			}
		}

		sort.Slice(items, func(i, j int) bool {
			return items[i].createdAt.After(items[j].createdAt)
		})

		renderPage(w, r, layout.Props{
			Title:       "Rollouts",
			CurrentPage: components.PageRollouts,
			Content:     page(items),
		})
	}
}

func page(rollouts []Summary) g.Node {
	return h.Div(
		h.Class("container"),
		h.Main(
			h.Class("main-content"),
			components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Rollouts()}),
			h.Div(
				h.Class("card"),
				h.Div(
					h.Class("card-body"),
					h.H1(g.Text("Rollouts")),
					rolloutsContent(rollouts),
				),
			),
		),
	)
}

func rolloutsContent(rollouts []Summary) g.Node {
	if len(rollouts) == 0 {
		return h.P(g.Text("No rollouts yet."))
	}

	return h.Table(
		h.Class("table sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Feature")),
			h.Th(g.Text("Version")),
			h.Th(g.Text("Status")),
			h.Th(g.Text("Created")),
			h.Th(g.Text("Completed")),
		)),
		h.TBody(g.Group(g.Map(rollouts, func(rollout Summary) g.Node {
			return h.Tr(
				h.Td(h.A(h.Href("/features/"+rollout.FeatureName), g.Text(rollout.FeatureName))),
				h.Td(versionCell(rollout)),
				h.Td(statusCell(rollout)),
				h.Td(timeWithTitle(rollout.Created)),
				h.Td(completedCell(rollout.Completed)),
			)
		}))),
	)
}

func versionCell(r Summary) g.Node {
	return h.A(h.Href("/rollouts/"+r.FeatureName+"/"+r.Version), g.Text(r.Version))
}

func statusCell(rollout Summary) g.Node {
	return rolloutStatus(rollout.Status)
}

func rolloutStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
	case "DISABLED":
		return g.Group([]g.Node{g.Text("⏸️ DISABLED")})
	default:
		return g.Text(status)
	}
}

func completedCell(t *time.Time) g.Node {
	if t == nil || t.IsZero() {
		return g.Text("-")
	}
	return timeWithTitle(*t)
}

func timeWithTitle(t time.Time) g.Node {
	if t.IsZero() {
		return g.Text("")
	}
	return h.Span(g.Attr("title", formatTime(t)), g.Text(relativeTime(t)))
}

func relativeTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < 0 {
		return formatTime(t)
	}
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d/time.Minute)) + "m ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d/time.Hour)) + "h ago"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 30*24*time.Hour:
		return strconv.Itoa(int(d/(24*time.Hour))) + "d ago"
	case d < 365*24*time.Hour:
		return strconv.Itoa(int(d/(30*24*time.Hour))) + "mo ago"
	default:
		return strconv.Itoa(int(d/(365*24*time.Hour))) + "y ago"
	}
}

var oslo = mustLoadLocation("Europe/Oslo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}

	return loc
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.In(oslo).Format("2006-01-02 15:04:05")
}
