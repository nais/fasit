package environments

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

const (
	healthyThreshold = 60 * time.Second
	staleThreshold   = 5 * time.Minute
)

type envRow struct {
	TenantName string
	EnvName    string
	ReportedAt time.Time
	HasReport  bool
}

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := uidata.ListTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
			return
		}

		rows := buildRows(r.Context(), tenants)

		query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		if query != "" {
			terms := strings.Fields(query)
			filtered := rows[:0]
			for _, row := range rows {
				text := strings.ToLower(row.TenantName + " " + row.EnvName)
				if matchesAll(text, terms) {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}

		renderPage(w, r, layout.Props{
			Title:       "Environments",
			CurrentPage: components.PageEnvironments,
			Content:     page(rows, query, time.Now()),
		})
	}
}

func buildRows(ctx context.Context, tenants []*uidata.Tenant) []envRow {
	var rows []envRow
	for _, tenant := range tenants {
		envs, err := tenant.Environments(ctx)
		if err != nil {
			continue
		}
		for _, env := range envs {
			row := envRow{
				TenantName: tenant.Name,
				EnvName:    env.Name,
			}
			health, err := naisdstatus.Get(ctx, env.ID)
			if err == nil && health.ReportedAt.Year() >= 2000 {
				row.ReportedAt = health.ReportedAt
				row.HasReport = true
			}
			rows = append(rows, row)
		}
	}
	return rows
}

func page(rows []envRow, query string, now time.Time) g.Node {
	return h.Div(
		h.Class("container"),
		components.Breadcrumbs([]breadcrumb.Crumb{breadcrumb.Environments()}),
		h.Main(
			h.Class("main-content"),
			h.Div(h.Class("card"), h.Div(
				h.Class("card-body"),
				h.Div(
					h.Class("assignments-header"),
					h.Div(
						h.Class("assignments-toolbar"),
						h.Input(
							h.Type("search"),
							h.Class("table-filter"),
							h.Name("q"),
							h.Placeholder("Filter by tenant, environment…"),
							g.Attr("aria-label", "Filter environments"),
							g.Attr("autocomplete", "off"),
							g.Attr("data-url-filter", "q"),
							g.If(query != "", h.Value(query)),
						),
					),
				),
				envTable(rows, now),
			)),
		),
	)
}

func envTable(rows []envRow, now time.Time) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No environments."))
	}

	tableRows := make([]g.Node, 0, len(rows))
	for _, row := range rows {
		hasLogo := components.HasTenantLogo(row.TenantName)
		tableRows = append(tableRows, h.Tr(
			h.Td(h.Span(
				h.Class("tenant-cell"),
				components.TenantAvatar(row.TenantName, hasLogo, "20px"),
				g.Text(row.TenantName),
			)),
			h.Td(h.A(h.Href("/tenants/"+row.TenantName+"/envs/"+row.EnvName), g.Text(row.EnvName))),
			h.Td(healthCell(row, now)),
		))
	}

	return h.Table(
		h.Class("table sortable"),
		g.Attr("data-sort-key", "environments"),
		h.THead(h.Tr(
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Agent health")),
		)),
		h.TBody(g.Group(tableRows)),
	)
}

func healthCell(row envRow, now time.Time) g.Node {
	class, label := agentHealth(row, now)

	title := "Never reported"
	if row.HasReport {
		title = "Last reported: " + view.FormatTime(row.ReportedAt)
	}

	return h.Span(h.Class("status-badge "+class), h.Title(title), g.Text(label))
}

func agentHealth(row envRow, now time.Time) (string, string) {
	if !row.HasReport {
		return "status-error", "no report"
	}
	age := now.Sub(row.ReportedAt)
	switch {
	case age < healthyThreshold:
		return "status-success", "healthy"
	case age < staleThreshold:
		return "status-pending", "stale"
	default:
		return "status-error", "dead"
	}
}

func matchesAll(text string, terms []string) bool {
	for _, term := range terms {
		if !strings.Contains(text, term) {
			return false
		}
	}
	return true
}
