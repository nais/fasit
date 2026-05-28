package reconcilerpage

import (
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, r, layout.Props{
			Title:       "Reconciler",
			CurrentPage: components.PageReconciler,
			Content:     triggerPage(),
		})
	}
}

func RunHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rec := reconciler.FromContext(r.Context())
		if rec == nil {
			http.Error(w, "reconciler not available", http.StatusInternalServerError)
			return
		}

		start := time.Now()
		result, err := rec.ComputeDesiredState(r.Context())
		elapsed := time.Since(start)
		if err != nil {
			http.Error(w, fmt.Sprintf("reconcile failed: %v", err), http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       "Reconciler",
			CurrentPage: components.PageReconciler,
			Content:     resultsPage(result.Decisions, elapsed, result.FetchDur, result.ComputeDur),
		})
	}
}

func triggerPage() g.Node {
	return h.Main(h.Class("main-content"),
		h.H1(g.Text("Reconciler")),
		h.P(h.Class("text-muted"), g.Text("Run a dry-run reconcile to see what would be deployed.")),
		h.Form(h.Method("post"), h.Action("/reconciler/run"),
			h.Button(h.Type("submit"), h.Class("btn"), g.Text("Run reconcile")),
		),
	)
}

type actionSummary struct {
	Deploy        int
	Unchanged     int
	InProgress    int
	Disabled      int
	Unhealthy     int
	MissingDeps   int
	MissingConfig int
	RenderError   int
}

func resultsPage(decisions []reconciler.DeployDecision, elapsed, fetchDur, computeDur time.Duration) g.Node {
	var summary actionSummary
	for _, d := range decisions {
		switch d.Action {
		case reconciler.ActionDeploy:
			summary.Deploy++
		case reconciler.ActionSkipUnchanged:
			summary.Unchanged++
		case reconciler.ActionSkipInProgress:
			summary.InProgress++
		case reconciler.ActionSkipDisabled:
			summary.Disabled++
		case reconciler.ActionSkipUnhealthy:
			summary.Unhealthy++
		case reconciler.ActionFailMissingDeps:
			summary.MissingDeps++
		case reconciler.ActionFailMissingConfig:
			summary.MissingConfig++
		case reconciler.ActionFailRender:
			summary.RenderError++
		}
	}

	sort.Slice(decisions, func(i, j int) bool {
		if decisions[i].Action != decisions[j].Action {
			return actionOrder(decisions[i].Action) < actionOrder(decisions[j].Action)
		}
		if decisions[i].TenantName != decisions[j].TenantName {
			return decisions[i].TenantName < decisions[j].TenantName
		}
		if decisions[i].EnvironmentName != decisions[j].EnvironmentName {
			return decisions[i].EnvironmentName < decisions[j].EnvironmentName
		}
		return decisions[i].Feature.Name < decisions[j].Feature.Name
	})

	return h.Main(h.Class("main-content"),
		h.Div(h.Class("reconciler-header"),
			h.H1(g.Text("Reconciler results")),
			h.Form(h.Method("post"), h.Action("/reconciler/run"),
				h.Button(h.Type("submit"), h.Class("btn"), g.Text("Run again")),
			),
		),
		h.Div(h.Class("reconciler-summary"),
			components.Card(timingTable(elapsed, fetchDur, computeDur)),
			components.Card(summaryTable(summary, len(decisions))),
		),
		components.Card(decisionsTable(decisions)),
	)
}

func actionOrder(a reconciler.Action) int {
	switch a {
	case reconciler.ActionFailRender:
		return 0
	case reconciler.ActionFailMissingConfig:
		return 1
	case reconciler.ActionFailMissingDeps:
		return 2
	case reconciler.ActionDeploy:
		return 3
	case reconciler.ActionSkipDisabled:
		return 4
	case reconciler.ActionSkipUnhealthy:
		return 5
	case reconciler.ActionSkipInProgress:
		return 6
	case reconciler.ActionSkipUnchanged:
		return 7
	default:
		return 8
	}
}

func fmtDur(d time.Duration) string {
	if d < time.Millisecond {
		return d.Round(time.Microsecond).String()
	}
	return d.Round(time.Millisecond).String()
}

func pct(part, total time.Duration) string {
	if total == 0 {
		return ""
	}
	return fmt.Sprintf("%.0f%%", float64(part)/float64(total)*100)
}

func timingTable(total, fetch, compute time.Duration) g.Node {
	return h.Table(h.Class("table table-compact"),
		h.THead(h.Tr(
			h.Th(g.Text("Phase")),
			h.Th(g.Text("Duration")),
			h.Th(g.Text("%")),
		)),
		h.TBody(
			h.Tr(h.Td(g.Text("Fetch")), h.Td(g.Text(fmtDur(fetch))), h.Td(g.Text(pct(fetch, total)))),
			h.Tr(h.Td(g.Text("Compute")), h.Td(g.Text(fmtDur(compute))), h.Td(g.Text(pct(compute, total)))),
			h.Tr(h.Class("timing-total"),
				h.Td(g.Text("Total")),
				h.Td(g.Text(fmtDur(total))),
				h.Td(),
			),
		),
	)
}

func summaryTable(s actionSummary, total int) g.Node {
	return h.Table(h.Class("table table-compact"),
		h.THead(h.Tr(
			h.Th(g.Text("Action")), h.Th(g.Text("Count")),
		)),
		h.TBody(
			summaryRow("Would deploy", s.Deploy, "status-success"),
			summaryRow("Unchanged", s.Unchanged, ""),
			summaryRow("In progress", s.InProgress, "status-pending"),
			summaryRow("Disabled", s.Disabled, "status-disabled"),
			summaryRow("Unhealthy naisd", s.Unhealthy, "status-disabled"),
			g.If(s.MissingDeps > 0, summaryRow("Missing dependencies", s.MissingDeps, "status-error")),
			g.If(s.MissingConfig > 0, summaryRow("Missing config", s.MissingConfig, "status-error")),
			g.If(s.RenderError > 0, summaryRow("Render error", s.RenderError, "status-error")),
			h.Tr(h.Class("timing-total"),
				h.Td(g.Text("Total")),
				h.Td(g.Textf("%d", total)),
			),
		),
	)
}

func summaryRow(label string, count int, class string) g.Node {
	return h.Tr(
		h.Td(g.If(class != "", h.Class(class)), g.Text(label)),
		h.Td(g.Textf("%d", count)),
	)
}

func decisionsTable(decisions []reconciler.DeployDecision) g.Node {
	if len(decisions) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No decisions."))
	}
	return h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "reconciler-decisions"),
		h.THead(h.Tr(
			h.Th(g.Text("Action")),
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Feature")),
			h.Th(g.Text("Version")),
			h.Th(g.Text("Message"), g.Attr("data-no-sort", "")),
		)),
		h.TBody(g.Map(decisions, func(d reconciler.DeployDecision) g.Node {
			return decisionRow(d)
		})...),
	)
}

func actionBadge(a reconciler.Action) g.Node {
	class := "status-badge "
	switch {
	case a == reconciler.ActionDeploy:
		class += "status-success"
	case a.IsFailure():
		class += "status-error"
	case a == reconciler.ActionSkipDisabled || a == reconciler.ActionSkipUnhealthy:
		class += "status-disabled"
	case a == reconciler.ActionSkipInProgress:
		class += "status-pending"
	default:
		class += ""
	}
	return h.Span(h.Class(class), g.Text(a.String()))
}

func decisionRow(d reconciler.DeployDecision) g.Node {
	return h.Tr(
		h.Td(g.Attr("data-sort-value", fmt.Sprintf("%d", actionOrder(d.Action))), actionBadge(d.Action)),
		h.Td(g.Text(d.TenantName)),
		h.Td(g.Text(d.EnvironmentName)),
		h.Td(g.Text(d.Feature.Name)),
		h.Td(g.Text(d.Feature.Version)),
		h.Td(g.Text(d.Message)),
	)
}
