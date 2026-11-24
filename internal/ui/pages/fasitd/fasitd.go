package fasitdpage

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/fasitd"
	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

const recentDisconnectsShown = 20

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

// StatusSource is the Fasit-side fasitd session server. Session state is held
// in memory by the Fasit instance that owns the gRPC streams.
type StatusSource interface {
	Overview(ctx context.Context) ([]fasitd.EnvironmentOverview, error)
	EnvironmentOverview(ctx context.Context, envID uuid.UUID, tenant, environment string) (fasitd.EnvironmentOverview, error)
}

var outcomeOrder = []commandstatus.Status{
	commandstatus.Deployed,
	commandstatus.Uninstalled,
	commandstatus.Failed,
	commandstatus.Undeliverable,
	commandstatus.Installing,
	commandstatus.Sent,
}

func Handler(renderPage RenderPage, src StatusSource) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		overviews, err := src.Overview(r.Context())
		if err != nil {
			http.Error(w, "Failed to load fasitd status: "+err.Error(), http.StatusInternalServerError)
			return
		}
		renderPage(w, r, layout.Props{
			Title:       "fasitd",
			CurrentPage: components.PageFasitd,
			Content:     overviewPage(overviews),
		})
	}
}

func overviewPage(overviews []fasitd.EnvironmentOverview) g.Node {
	return h.Main(
		h.Class("main-content"),
		h.H1(g.Text("fasitd")),
		h.P(h.Class("text-muted"), g.Textf(
			"Live sessions held by this Fasit instance. Command outcomes cover the last %s.", formatWindow(fasitd.OutcomeWindow))),
		h.Div(h.Class("card"), h.Div(h.Class("card-body"),
			h.Table(
				h.Class("table sortable"), g.Attr("data-sort-key", "fasitd-overview"),
				h.THead(h.Tr(
					h.Th(g.Text("Environment")),
					h.Th(g.Text("Sessions")),
					h.Th(g.Text("Primary")),
					h.Th(g.Text("Last heartbeat")),
					h.Th(g.Text("Commands")),
					h.Th(g.Text("Release inventory")),
				)),
				h.TBody(g.Map(overviews, overviewRow)),
			),
		)),
		recentDisconnectsCard(overviews),
	)
}

func overviewRow(ov fasitd.EnvironmentOverview) g.Node {
	primary, hasPrimary := ov.Primary()
	return h.Tr(
		h.Td(h.A(h.Href(EnvironmentHref(ov.Tenant, ov.Environment)), g.Text(ov.Tenant+" / "+ov.Environment))),
		h.Td(g.Attr("data-sort-value", fmt.Sprint(len(ov.Sessions))), sessionCount(ov.Sessions)),
		h.Td(g.If(hasPrimary, g.Group([]g.Node{g.Text(primary.Version), instanceSuffix(primary.Instance)}))),
		g.If(hasPrimary, view.TimeCell(primary.LastHeartbeat)),
		g.If(!hasPrimary, h.Td()),
		h.Td(outcomes(ov.Outcomes)),
		h.Td(releaseSummary(ov.Releases)),
	)
}

func EnvironmentHref(tenant, environment string) string {
	return "/tenants/" + tenant + "/envs/" + environment + "?tab=fasitd"
}

func sessionCount(sessions []fasitd.SessionStatus) g.Node {
	if len(sessions) == 0 {
		return h.Span(h.Class("badge status-error"), g.Text("none"))
	}
	counts := map[fasitd.SessionRole]int{}
	for _, s := range sessions {
		counts[s.Role]++
	}
	return g.Group([]g.Node{
		g.Textf("%d", len(sessions)),
		g.If(counts[fasitd.RolePrimary] == 0, g.Group([]g.Node{g.Text(" "), h.Span(h.Class("badge status-error"), g.Text("no primary"))})),
		g.If(counts[fasitd.RoleDraining] > 0, g.Group([]g.Node{g.Text(" "), h.Span(h.Class("badge status-warning"), g.Textf("%d draining", counts[fasitd.RoleDraining]))})),
	})
}

func outcomes(counts map[commandstatus.Status]int64) g.Node {
	var parts []g.Node
	for _, s := range outcomeOrder {
		if n := counts[s]; n > 0 {
			parts = append(parts, h.Span(h.Class("fasitd-outcome "+outcomeClass(s)), h.Title(string(s)), g.Textf("%d %s", n, s)))
		}
	}
	for _, s := range slices.Sorted(maps.Keys(counts)) {
		if !slices.Contains(outcomeOrder, s) {
			parts = append(parts, h.Span(h.Class("fasitd-outcome"), g.Textf("%d %s", counts[s], s)))
		}
	}
	if len(parts) == 0 {
		return h.Span(h.Class("text-muted"), g.Text("none"))
	}
	return g.Group(parts)
}

func outcomeClass(s commandstatus.Status) string {
	switch s {
	case commandstatus.Deployed, commandstatus.Uninstalled:
		return "status-success"
	case commandstatus.Failed:
		return "status-error"
	case commandstatus.Undeliverable:
		return "status-warning"
	default:
		return "text-muted"
	}
}

func releaseSummary(r fasitd.ReleaseSummary) g.Node {
	if !r.Reported() {
		return h.Span(h.Class("text-muted"), g.Text("never reported"))
	}
	return g.Group([]g.Node{
		g.Textf("%d releases, ", r.Count),
		h.Span(h.Title(view.FormatTime(r.ReportedAt)), g.Text(view.RelativeTime(r.ReportedAt))),
		g.Text(" "),
		g.If(r.Differences() == 0, h.Span(h.Class("badge status-success"), g.Text("matches naisd"))),
		g.If(r.Differences() > 0, h.Span(h.Class("badge status-warning"), g.Textf("%d differ from naisd", r.Differences()))),
	})
}

type disconnect struct {
	fasitd.ClosedSession
	Tenant      string
	Environment string
}

func recentDisconnectsCard(overviews []fasitd.EnvironmentOverview) g.Node {
	var all []disconnect
	for _, ov := range overviews {
		for _, c := range ov.Closed {
			all = append(all, disconnect{ClosedSession: c, Tenant: ov.Tenant, Environment: ov.Environment})
		}
	}
	slices.SortFunc(all, func(a, b disconnect) int { return b.ClosedAt.Compare(a.ClosedAt) })
	if len(all) > recentDisconnectsShown {
		all = all[:recentDisconnectsShown]
	}
	return h.Div(h.Class("card"), h.Div(h.Class("card-body"),
		h.H2(h.Class("card-section-heading"), g.Text("Recent disconnects")),
		g.If(len(all) == 0, h.P(h.Class("text-muted"), g.Text("No sessions have closed since Fasit started."))),
		g.If(len(all) > 0, h.Table(
			h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Closed")),
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Instance")),
				h.Th(g.Text("Version")),
				h.Th(g.Text("Duration")),
				h.Th(g.Text("Drained")),
				h.Th(g.Text("Reason")),
			)),
			h.TBody(g.Map(all, func(d disconnect) g.Node {
				return h.Tr(
					view.TimeCell(d.ClosedAt),
					h.Td(h.A(h.Href(EnvironmentHref(d.Tenant, d.Environment)), g.Text(d.Tenant+" / "+d.Environment))),
					h.Td(textOrMuted(d.Instance, "unknown")),
					h.Td(g.Text(d.Version)),
					h.Td(g.Text(formatDuration(d.ClosedAt.Sub(d.ConnectedAt)))),
					h.Td(g.Text(yesNo(d.Drained))),
					h.Td(g.Text(d.Reason)),
				)
			})),
		)),
	))
}

// EnvironmentContent renders the fasitd tab of an environment page.
func EnvironmentContent(ov fasitd.EnvironmentOverview) g.Node {
	return g.Group([]g.Node{
		h.Div(h.Class("card"), h.Div(h.Class("card-body"),
			h.H2(h.Class("card-section-heading"), g.Text("fasitd")),
			h.P(h.Class("text-muted"), g.Text("Commands go to the first session that accepts them; standbys take over when the primary drains or disconnects.")),
			h.Div(
				h.Class("environment-details-list"),
				detailsItem("Sessions", sessionCount(ov.Sessions)),
				detailsItem("Commands, last "+formatWindow(fasitd.OutcomeWindow), outcomes(ov.Outcomes)),
				detailsItem("Release inventory", releaseSummary(ov.Releases)),
			),
		)),
		g.Map(ov.Sessions, sessionCard),
		releaseDifferencesCard(ov.Releases),
		environmentDisconnectsCard(ov.Closed),
	})
}

func sessionCard(s fasitd.SessionStatus) g.Node {
	return h.Div(h.Class("card"), h.Div(h.Class("card-body"),
		h.H2(h.Class("card-section-heading"),
			g.Text(cmp.Or(s.Instance, "unknown instance")+" "),
			h.Span(h.Class("badge "+roleClass(s.Role)), g.Text(string(s.Role))),
		),
		h.Div(
			h.Class("environment-details-list"),
			detailsItem("Version", g.Text(s.Version)),
			detailsItem("Session ID", h.Code(g.Text(s.ID.String()))),
			detailsItem("Connected", timeText(s.ConnectedAt)),
			detailsItem("Last message", timeText(s.LastSeen)),
			detailsItem("Last heartbeat", timeText(s.LastHeartbeat)),
			g.If(!s.DrainingSince.IsZero(), detailsItem("Draining since", timeText(s.DrainingSince))),
			detailsItem("Queued commands", g.Textf("%d", s.QueueDepth)),
			detailsItem("Received", counters(s.MessagesIn)),
			detailsItem("Sent", counters(s.MessagesOut)),
		),
		h.H3(g.Text("Recent messages")),
		g.If(len(s.Recent) == 0, h.P(h.Class("text-muted"), g.Text("No messages besides heartbeats yet."))),
		g.If(len(s.Recent) > 0, h.Table(
			h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Time")),
				h.Th(g.Text("")),
				h.Th(g.Text("Type")),
				h.Th(g.Text("Summary")),
			)),
			h.TBody(g.Map(s.Recent, func(m fasitd.MessageRecord) g.Node {
				return h.Tr(
					view.TimeCell(m.Time),
					h.Td(h.Title(directionTitle(m.Direction)), g.Text(directionArrow(m.Direction))),
					h.Td(g.Text(m.Type)),
					h.Td(g.Text(m.Summary)),
				)
			})),
		)),
	))
}

func releaseDifferencesCard(r fasitd.ReleaseSummary) g.Node {
	if !r.Reported() || r.Differences() == 0 {
		return nil
	}
	rows := make([]g.Node, 0, r.Differences())
	for _, name := range r.OnlyFasitd {
		rows = append(rows, h.Tr(h.Td(g.Text(name)), h.Td(g.Text("only reported by fasitd")), h.Td(), h.Td()))
	}
	for _, name := range r.OnlyNaisd {
		rows = append(rows, h.Tr(h.Td(g.Text(name)), h.Td(g.Text("only reported by naisd")), h.Td(), h.Td()))
	}
	for _, m := range r.Mismatched {
		rows = append(rows, h.Tr(
			h.Td(g.Text(m.Name)),
			h.Td(g.Text("differs")),
			h.Td(g.Text(m.FasitdVersion+" ("+m.FasitdStatus+")")),
			h.Td(g.Text(m.NaisdVersion+" ("+m.NaisdStatus+")")),
		))
	}
	return h.Div(h.Class("card"), h.Div(h.Class("card-body"),
		h.H2(h.Class("card-section-heading"), g.Text("Release inventory vs naisd")),
		h.Table(
			h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Release")),
				h.Th(g.Text("Difference")),
				h.Th(g.Text("fasitd")),
				h.Th(g.Text("naisd")),
			)),
			h.TBody(rows...),
		),
	))
}

func environmentDisconnectsCard(closed []fasitd.ClosedSession) g.Node {
	return h.Div(h.Class("card"), h.Div(h.Class("card-body"),
		h.H2(h.Class("card-section-heading"), g.Text("Recent disconnects")),
		g.If(len(closed) == 0, h.P(h.Class("text-muted"), g.Text("No sessions have closed since Fasit started."))),
		g.If(len(closed) > 0, h.Table(
			h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Closed")),
				h.Th(g.Text("Instance")),
				h.Th(g.Text("Version")),
				h.Th(g.Text("Duration")),
				h.Th(g.Text("Messages in/out")),
				h.Th(g.Text("Drained")),
				h.Th(g.Text("Reason")),
			)),
			h.TBody(g.Map(closed, func(c fasitd.ClosedSession) g.Node {
				return h.Tr(
					view.TimeCell(c.ClosedAt),
					h.Td(textOrMuted(c.Instance, "unknown")),
					h.Td(g.Text(c.Version)),
					h.Td(g.Text(formatDuration(c.ClosedAt.Sub(c.ConnectedAt)))),
					h.Td(g.Textf("%d / %d", c.MessagesIn, c.MessagesOut)),
					h.Td(g.Text(yesNo(c.Drained))),
					h.Td(g.Text(c.Reason)),
				)
			})),
		)),
	))
}

func detailsItem(key string, value g.Node) g.Node {
	return h.Div(
		h.Class("environment-details-item"),
		h.Div(h.Class("environment-details-key"), g.Text(key)),
		h.Div(h.Class("environment-details-value"), value),
	)
}

func counters(m map[string]int64) g.Node {
	if len(m) == 0 {
		return h.Span(h.Class("text-muted"), g.Text("none"))
	}
	parts := make([]string, 0, len(m))
	for _, k := range slices.Sorted(maps.Keys(m)) {
		parts = append(parts, fmt.Sprintf("%s %d", k, m[k]))
	}
	return g.Text(strings.Join(parts, ", "))
}

func timeText(t time.Time) g.Node {
	if t.IsZero() {
		return h.Span(h.Class("text-muted"), g.Text("never"))
	}
	return h.Span(h.Title(view.FormatTime(t)), g.Text(view.RelativeTime(t)))
}

func roleClass(r fasitd.SessionRole) string {
	switch r {
	case fasitd.RolePrimary:
		return "status-success"
	case fasitd.RoleDraining:
		return "status-warning"
	default:
		return ""
	}
}

func instanceSuffix(instance string) g.Node {
	if instance == "" {
		return nil
	}
	return h.Span(h.Class("text-muted"), g.Text(" "+instance))
}

func directionArrow(d fasitd.Direction) string {
	if d == fasitd.DirectionOut {
		return "→"
	}
	return "←"
}

func directionTitle(d fasitd.Direction) string {
	if d == fasitd.DirectionOut {
		return "Fasit to fasitd"
	}
	return "fasitd to Fasit"
}

func textOrMuted(value, fallback string) g.Node {
	if value == "" {
		return h.Span(h.Class("text-muted"), g.Text(fallback))
	}
	return g.Text(value)
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return d.Round(time.Second).String()
	}
	return d.Round(time.Minute).String()
}

func formatWindow(d time.Duration) string {
	if d%time.Hour == 0 {
		return fmt.Sprintf("%dh", d/time.Hour)
	}
	return d.String()
}
