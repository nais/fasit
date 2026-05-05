package naisd

import (
	"net/http"
	"sort"
	"time"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

const (
	healthyThreshold = 60 * time.Second
	staleThreshold   = 5 * time.Minute
)

type agentRow struct {
	Tenant      string
	Environment string
	ReportedAt  time.Time
	HasReport   bool
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := environment.GetTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
			return
		}

		var rows []agentRow
		for _, tenant := range tenants {
			envs, err := repo.EnvironmentsGet(r.Context(), tenant.ID)
			if err != nil {
				continue
			}
			for _, env := range envs {
				row := agentRow{Tenant: tenant.Name, Environment: env.Name}
				health, err := naisdstatus.Get(r.Context(), env.ID)
				if err == nil && !isSentinel(health.ReportedAt) {
					row.ReportedAt = health.ReportedAt
					row.HasReport = true
				}
				rows = append(rows, row)
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Tenant != rows[j].Tenant {
				return rows[i].Tenant < rows[j].Tenant
			}
			return rows[i].Environment < rows[j].Environment
		})

		renderPage(w, r, layout.Props{
			Title:       "Naisd",
			CurrentPage: components.PageNaisd,
			Content:     page(rows, time.Now()),
		})
	}
}

func isSentinel(t time.Time) bool {
	return t.Year() < 2000
}

func page(rows []agentRow, now time.Time) g.Node {
	return h.Main(h.Class("main-content"),
		h.H1(g.Text("Naisd agents")),
		h.P(h.Class("text-muted"),
			g.Textf("%d agents · healthy < %s · stale < %s · dead beyond that",
				len(rows), healthyThreshold, staleThreshold),
		),
		h.Table(h.Class("table sortable"),
			h.THead(h.Tr(
				h.Th(g.Text("Tenant")),
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Health")),
				h.Th(g.Text("Last reported")),
				h.Th(g.Text("Age")),
			)),
			h.TBody(g.Group(g.Map(rows, func(row agentRow) g.Node {
				return h.Tr(
					h.Td(g.Text(row.Tenant)),
					h.Td(g.Text(row.Environment)),
					h.Td(healthCell(row, now)),
					h.Td(g.Text(lastReportedCell(row))),
					h.Td(g.Text(ageCell(row, now))),
				)
			}))),
		),
	)
}

func healthCell(row agentRow, now time.Time) g.Node {
	class, label := healthBucket(row, now)
	return h.Span(h.Class("status-badge "+class), g.Text(label))
}

func healthBucket(row agentRow, now time.Time) (string, string) {
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

func lastReportedCell(row agentRow) string {
	if !row.HasReport {
		return "never"
	}
	return view.FormatTime(row.ReportedAt)
}

func ageCell(row agentRow, now time.Time) string {
	if !row.HasReport {
		return "-"
	}
	age := now.Sub(row.ReportedAt).Round(time.Second)
	return age.String() + " ago"
}

