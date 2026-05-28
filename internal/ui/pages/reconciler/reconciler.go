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
	return h.Div(h.Class("content"),
		h.H1(g.Text("Reconciler")),
		h.P(g.Text("Run a dry-run reconcile to see what would be deployed.")),
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

func resultsPage(results []reconciler.DeployDecision, elapsed, fetchDur, computeDur time.Duration) g.Node {
	var summary actionSummary
	byEnv := map[string][]reconciler.DeployDecision{}

	for _, r := range results {
		key := r.TenantName + "/" + r.EnvironmentName
		byEnv[key] = append(byEnv[key], r)

		switch r.Action {
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

	envKeys := make([]string, 0, len(byEnv))
	for k := range byEnv {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	return h.Div(h.Class("content"),
		h.H1(g.Text("Reconciler results")),
		summarySection(summary, len(results), elapsed, fetchDur, computeDur),
		h.Form(h.Method("post"), h.Action("/reconciler/run"),
			h.Button(h.Type("submit"), h.Class("btn"), g.Text("Run again")),
		),
		g.If(summary.Deploy > 0 || summary.MissingDeps > 0 || summary.MissingConfig > 0 || summary.RenderError > 0,
			actionableResults(byEnv, envKeys),
		),
	)
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

func summarySection(s actionSummary, total int, elapsed, fetchDur, computeDur time.Duration) g.Node {
	return h.Div(h.Class("reconciler-summary"),
		timingTable(elapsed, fetchDur, computeDur),
		h.Table(h.Class("summary-table"),
			h.THead(h.Tr(
				h.Th(g.Text("Action")), h.Th(g.Text("Count")),
			)),
			h.TBody(
				summaryRow("Would deploy", s.Deploy, "action-deploy"),
				summaryRow("Unchanged", s.Unchanged, "action-skip"),
				summaryRow("In progress", s.InProgress, "action-skip"),
				summaryRow("Disabled", s.Disabled, "action-skip"),
				summaryRow("Unhealthy naisd", s.Unhealthy, "action-skip"),
				g.If(s.MissingDeps > 0, summaryRow("Missing dependencies", s.MissingDeps, "action-fail")),
				g.If(s.MissingConfig > 0, summaryRow("Missing config", s.MissingConfig, "action-fail")),
				g.If(s.RenderError > 0, summaryRow("Render error", s.RenderError, "action-fail")),
				summaryRow("Total", total, ""),
			),
		),
	)
}

func summaryRow(label string, count int, class string) g.Node {
	return h.Tr(
		g.If(class != "", h.Class(class)),
		h.Td(g.Text(label)),
		h.Td(g.Textf("%d", count)),
	)
}

func timingTable(total, fetch, compute time.Duration) g.Node {
	return h.Table(h.Class("timing-table"),
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

func actionableResults(byEnv map[string][]reconciler.DeployDecision, envKeys []string) g.Node {
	var sections []g.Node
	for _, key := range envKeys {
		results := byEnv[key]
		var actionable []reconciler.DeployDecision
		for _, r := range results {
			if r.Action == reconciler.ActionDeploy || r.Action.IsFailure() {
				actionable = append(actionable, r)
			}
		}
		if len(actionable) == 0 {
			continue
		}
		sort.Slice(actionable, func(i, j int) bool {
			if actionable[i].Action != actionable[j].Action {
				return actionable[i].Action < actionable[j].Action
			}
			return actionable[i].Feature.Name < actionable[j].Feature.Name
		})
		sections = append(sections, envSection(key, actionable, len(results)))
	}
	return h.Div(h.Class("reconciler-details"), g.Group(sections))
}

func envSection(envKey string, results []reconciler.DeployDecision, total int) g.Node {
	return h.Details(
		h.Class("reconciler-env"),
		h.Summary(g.Textf("%s — %d actionable / %d total", envKey, len(results), total)),
		h.Table(h.Class("env-results-table"),
			h.THead(h.Tr(
				h.Th(g.Text("Feature")),
				h.Th(g.Text("Version")),
				h.Th(g.Text("Action")),
				h.Th(g.Text("Message")),
			)),
			h.TBody(g.Map(results, func(r reconciler.DeployDecision) g.Node {
				return resultRow(r)
			})...),
		),
	)
}

func resultRow(r reconciler.DeployDecision) g.Node {
	class := "action-skip"
	switch {
	case r.Action == reconciler.ActionDeploy:
		class = "action-deploy"
	case r.Action.IsFailure():
		class = "action-fail"
	}

	return h.Tr(h.Class(class),
		h.Td(g.Text(r.Feature.Name)),
		h.Td(g.Text(r.Feature.Version)),
		h.Td(g.Text(r.Action.String())),
		h.Td(g.Text(r.Message)),
	)
}
