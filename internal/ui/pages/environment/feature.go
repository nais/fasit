package environment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/dbtx"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/auditview"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeatureContextTabHandler(renderPage RenderPage, activeTab string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeaturePageData(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), activeTab)
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}
		data.ExpandedLogID = r.URL.Query().Get("logs")

		renderPage(w, r, layout.Props{
			Title:       data.Feature.Name + " / " + data.Tenant.Name + " / " + data.Environment.Name,
			CurrentPage: components.PageFeatures,
			Content:     featurePageContent(data),
		})
	}
}

func AuditRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		feature := chi.URLParam(r, "feature")
		tenant := chi.URLParam(r, "tenant")
		env := chi.URLParam(r, "env")
		query := url.QueryEscape(feature + " " + tenant + "/" + env)
		http.Redirect(w, r, "/auditlog?q="+query, http.StatusSeeOther)
	}
}

// FeatureLogsRedirectHandler redirects the legacy /logs URL to the merged
// feature page, where deploy logs are available via the Logs popover.
func FeatureLogsRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		basePath := "/features/" + chi.URLParam(r, "feature") + "/envs/" + chi.URLParam(r, "tenant") + "/" + chi.URLParam(r, "env")
		http.Redirect(w, r, basePath, http.StatusSeeOther)
	}
}

func UpdateConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		configID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid configuration id", http.StatusBadRequest)
			return
		}

		value, err := components.ParseConfigValue(r.FormValue("value"), r.FormValue("type"), r.FormValue("mode"))
		if err != nil {
			http.Error(w, "Invalid value format: "+err.Error(), http.StatusBadRequest)
			return
		}

		raw, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "Failed to encode value", http.StatusBadRequest)
			return
		}

		if err := dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			_, err := featurepkg.ConfigEnvUpdate(ctx, configID, featurepkg.UpdateConfiguration{Value: raw})
			return err
		}); err != nil {
			http.Error(w, "Failed to update configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()
		http.Redirect(w, r, featureBasePath(r)+"/config", http.StatusSeeOther)
	}
}

func DeleteConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid configuration id", http.StatusBadRequest)
			return
		}
		if err := dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			return featurepkg.ConfigEnvDelete(ctx, configID)
		}); err != nil {
			http.Error(w, "Failed to delete configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}
		reconciler.TriggerReconcile()
		http.Redirect(w, r, featureBasePath(r)+"/config", http.StatusSeeOther)
	}
}

func ConfigOverrideSubmitHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		tenant, err := envpkg.GetTenantByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		value, err := components.ParseConfigValue(r.FormValue("value"), r.FormValue("type"), r.FormValue("mode"))
		if err != nil {
			http.Error(w, "Invalid value format: "+err.Error(), http.StatusBadRequest)
			return
		}
		raw, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "Failed to encode value", http.StatusBadRequest)
			return
		}

		featureName := chi.URLParam(r, "feature")
		key := r.FormValue("key")

		feat, err := featureassignment.FeatureForEnvironment(r.Context(), env.ID, featureName)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, featureassignment.ErrFeatureNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, "Failed to get feature: "+err.Error(), status)
			return
		}
		secret := false
		if v, ok := feat.Values[key]; ok && v.Config != nil {
			secret = v.Config.Secret
		}

		err = dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			_, err := featurepkg.ConfigEnvCreate(ctx, featurepkg.NewConfiguration{
				EnvironmentID: &env.ID,
				Feature:       featureName,
				Key:           key,
				Value:         raw,
				Secret:        secret,
			})
			return err
		})
		if err != nil {
			http.Error(w, "Failed to create configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()
		http.Redirect(w, r, featureBasePath(r)+"/config", http.StatusSeeOther)
	}
}

func BatchUpdateConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		tenant, err := envpkg.GetTenantByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		featureName := chi.URLParam(r, "feature")
		feat, err := featureassignment.FeatureForEnvironment(r.Context(), env.ID, featureName)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, featureassignment.ErrFeatureNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, "Failed to get feature: "+err.Error(), status)
			return
		}

		changed := 0
		err = dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			for _, key := range r.PostForm[components.BulkKeysField] {
				newValue := r.PostFormValue(components.BulkValueField(key))
				if newValue == r.PostFormValue(components.BulkOrigField(key)) {
					continue
				}
				value, err := components.ParseConfigValue(newValue, r.PostFormValue(components.BulkTypeField(key)), "raw")
				if err != nil {
					return fmt.Errorf("%s: invalid value format: %w", key, err)
				}
				raw, err := json.Marshal(value)
				if err != nil {
					return fmt.Errorf("%s: encode value: %w", key, err)
				}

				if id := r.PostFormValue(components.BulkIDField(key)); id != "" {
					configID, err := uuid.Parse(id)
					if err != nil {
						return fmt.Errorf("%s: invalid configuration id: %w", key, err)
					}
					if _, err := featurepkg.ConfigEnvUpdate(ctx, configID, featurepkg.UpdateConfiguration{Value: raw}); err != nil {
						return fmt.Errorf("%s: %w", key, err)
					}
				} else {
					secret := false
					if v, ok := feat.Values[key]; ok && v.Config != nil {
						secret = v.Config.Secret
					}
					if _, err := featurepkg.ConfigEnvCreate(ctx, featurepkg.NewConfiguration{
						EnvironmentID: &env.ID,
						Feature:       featureName,
						Key:           key,
						Value:         raw,
						Secret:        secret,
					}); err != nil {
						return fmt.Errorf("%s: %w", key, err)
					}
				}
				changed++
			}
			return nil
		})
		if err != nil {
			http.Error(w, "Failed to update configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if changed > 0 {
			reconciler.TriggerReconcile()
		}
		http.Redirect(w, r, featureBasePath(r)+"/config", http.StatusSeeOther)
	}
}

func ToggleFeatureStateHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		tenant, err := envpkg.GetTenantByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		feature, err := featureassignment.FeatureForEnvironment(r.Context(), env.ID, chi.URLParam(r, "feature"))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, featureassignment.ErrFeatureNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, "Failed to get feature: "+err.Error(), status)
			return
		}
		enabled := r.FormValue("enabled") == "true"
		var reason string
		if !enabled {
			reason = strings.TrimSpace(r.FormValue("reason"))
			if reason == "" {
				http.Error(w, "A reason is required to disable reconcile.", http.StatusBadRequest)
				return
			}
			if len(reason) > 1000 {
				http.Error(w, "Reason must be at most 1000 characters.", http.StatusBadRequest)
				return
			}
		}

		if enabled {
			err = featurepkg.FeatureEnable(r.Context(), env.ID, feature.Name)
		} else {
			err = featurepkg.FeatureDisable(r.Context(), env.ID, feature.Name, reason)
		}
		if err != nil {
			http.Error(w, "Failed to toggle feature state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		reconciler.TriggerReconcile()
		http.Redirect(w, r, redirectOrDefault(r, featureBasePath(r)), http.StatusSeeOther)
	}
}

func RedeployHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, err := envpkg.GetTenantByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		feature, err := featureassignment.FeatureForEnvironment(r.Context(), env.ID, chi.URLParam(r, "feature"))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, featureassignment.ErrFeatureNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, "Failed to get feature: "+err.Error(), status)
			return
		}

		rec := reconciler.FromContext(r.Context())
		if rec == nil {
			http.Error(w, "Reconciler unavailable", http.StatusInternalServerError)
			return
		}

		switch err := rec.Redeploy(r.Context(), env.ID, feature.Name); {
		case err == nil:
		case errors.Is(err, reconciler.ErrRedeployNotSettled),
			errors.Is(err, reconciler.ErrRedeployTargetNotFound):
			// The UI grays out redeploy unless it is allowed, so reaching here
			// means the state changed between render and click. Bounce back to
			// the feature page, which now reflects the current state.
			http.Redirect(w, r, redirectOrDefault(r, featureBasePath(r)), http.StatusSeeOther)
			return
		default:
			http.Error(w, "Failed to redeploy: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := audit.Create(r.Context(), audit.CreateParams{
			Action:        audit.ActionRedeploy,
			ObjectType:    audit.ObjectTypeFeature,
			ObjectID:      feature.Name,
			Feature:       feature.Name,
			EnvironmentID: &env.ID,
		}); err != nil {
			http.Error(w, "Failed to record audit entry: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, redirectOrDefault(r, featureBasePath(r)), http.StatusSeeOther)
	}
}

func featurePageContent(page *FeaturePage) g.Node {
	merged := page.ActiveTab != "assignments"
	var headerLeft, body g.Node
	if merged {
		headerLeft = deployHeaderLeft(page)
		body = h.Div(h.Class("tab-content-wrapper env-feature-merged"), overviewTab(page))
	} else {
		headerLeft = h.Div()
		body = assignmentsTab(page)
	}

	return h.Div(h.Class("container"),
		featurePageSidebar(page),
		components.Breadcrumbs(page.Breadcrumbs),
		h.Main(h.Class("main-content"),
			components.Card(
				h.Div(h.Class("card-header-row"),
					headerLeft,
					h.Div(h.Class("card-header-actions"),
						reconcileHeaderStatus(page),
						pageKebab(page),
					),
				),
				body,
			),
		),
		envActivitySidebar(page),
	)
}

func featurePageSidebar(page *FeaturePage) g.Node {
	return components.FeatureSidebar(page.Feature.Name, "", page.TenantSlug, page.Environment.Name, page.FeatureEnvs)
}

func pageKebab(page *FeaturePage) g.Node {
	kebabID := "page-kebab"
	items := []g.Node{}

	items = append(items, components.LokiLogsItem(LokiExploreURL(page.Tenant.Name, page.Environment.Name, page.Feature.Name)))
	items = append(items,
		h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("data-lazy-modal", featureBasePathForPage(page)+"/helm-values"),
			g.Raw(components.IconDocument),
			g.Text("Render Helm values"),
		),
	)
	reconcilePopoverID := "toggle-reconcile"

	if page.Feature.Enabled {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item kebab-item-danger"), g.Attr("popovertarget", reconcilePopoverID),
				g.Raw(components.IconPause),
				g.Text("Disable reconcile"),
			),
		)
	} else {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", reconcilePopoverID),
				g.Raw(components.IconPlay),
				g.Text("Enable reconcile"),
			),
		)
	}

	if page.WinningAssignment != nil {
		items = append(items,
			h.A(h.Href("/assignments/"+page.WinningAssignment.ID.String()), h.Class("kebab-item"),
				g.Raw(components.IconDocument),
				g.Text("View assignment"),
			),
		)
	}

	if page.WinningAssignment != nil {
		if isRedeployable(page) {
			items = append(items,
				h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", "redeploy"),
					g.Raw(components.IconRedeploy),
					g.Text("Redeploy"),
				),
			)
		} else {
			items = append(items,
				h.Button(h.Type("button"), h.Class("kebab-item kebab-item-disabled"), g.Attr("disabled", ""),
					h.Title("Redeploy is only available when the feature is enabled and in a settled state (deployed or failed)"),
					g.Raw(components.IconRedeploy),
					g.Text("Redeploy"),
				),
			)
		}
	}

	if len(page.RecentDeployHistory) > 0 {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", "deploy-history"),
				g.Raw(components.IconHistory),
				g.Text("Deploy history"),
			),
		)
	}

	if len(page.DecisionHistory) > 0 {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", "decision-history"),
				g.Raw(components.IconLogs),
				g.Text("Reconciler log"),
			),
		)
	}

	return components.KebabWrap(kebabID, items, reconcilePopover(page), redeployPopover(page), decisionHistoryPopover(page), deployHistoryPopover(page), deployLogsPopover(page))
}

func reconcilePopover(page *FeaturePage) g.Node {
	return components.ReconcilePopover("toggle-reconcile", featureBasePathForPage(page)+"/toggle-reconcile", page.Feature.Name, page.Environment.Name, page.Feature.Enabled, "")
}

func redeployPopover(page *FeaturePage) g.Node {
	return components.RedeployPopover("redeploy", featureBasePathForPage(page)+"/redeploy", page.Feature.Name, page.Environment.Name, isRedeployable(page), "")
}

// isRedeployable reports whether a manual redeploy can be triggered now: the
// feature must be enabled and in a settled, terminal state. This mirrors the
// server-side gate in Reconciler.Redeploy (which only proceeds from
// ActionSkipUnchanged); the handler re-checks, so a stale render is harmless.
func isRedeployable(page *FeaturePage) bool {
	return page.Feature.Enabled && (page.Status == "DEPLOYED" || page.Status == "FAILED")
}

// deployHeaderLeft is the left side of the merged page's header bar: any disabled
// reason, an actionable problem (failed render, missing config/deps, unhealthy
// agent) and the latest deploy with links to logs and full history. The popovers
// these trigger are emitted by pageKebab.
// deployHeaderLeft stacks two independent truths the page must not conflate:
// the runtime line (the Helm release naisd reports from the cluster) and, only
// when Fasit has not converged, a reconcile alert (an in-flight, failed, or
// blocked deploy attempt). A failed deploy does not remove the installed
// release, so both lines can be present at once.
func deployHeaderLeft(page *FeaturePage) g.Node {
	rows := []g.Node{}
	if !page.Feature.Enabled {
		reason := page.Feature.DisableReason
		if reason == "" {
			reason = "disabled before we started requiring reason"
		}
		rows = append(rows, h.Div(h.Class("reconcile-reason-inline"), g.Text(reason)))
	}
	rows = append(rows, runtimeLine(page))
	if alert := reconcileAlertLine(page); alert != nil {
		rows = append(rows, alert)
	}
	return h.Div(h.Class("deploy-status-stack"), g.Group(rows))
}

// runtimeLine reports what is actually installed in the cluster, as reported by
// naisd. It says "Installed" rather than "Running" because a Helm release may
// contain nothing that runs (e.g. just a ConfigMap). When the feature is
// converged the deploy log is offered here, since the last deploy is the one
// that produced the installed release.
func runtimeLine(page *FeaturePage) g.Node {
	rel := page.Release
	if rel == nil {
		return h.Div(h.Class("runtime-line text-muted"), g.Text("No release reported."))
	}

	var status g.Node
	if strings.EqualFold(rel.Status, "deployed") {
		status = h.Span(h.Class("status-success"), g.Text("✓ Installed"))
	} else {
		status = components.Status(strings.ToUpper(rel.Status))
	}

	var logs g.Node
	if isConverged(page) {
		logs = logsLink(page, "Logs")
	}

	return h.Div(h.Class("runtime-line"),
		h.Span(h.Class("deploy-version"), g.Text(rel.Version)),
		status,
		h.Span(h.Class("text-muted"), h.Title(view.FormatTime(rel.LastDeployed)), g.Text("updated "+view.RelativeTime(rel.LastDeployed))),
		logs,
	)
}

// reconcileAlertLine surfaces Fasit's deploy intent only when it has not
// converged: a deploy in flight, a failed deploy, or a pre-deploy block. It
// returns nil when the latest desired state is installed (the common case), so
// the header stays quiet. The failed caption is deliberately generic: the
// underlying decision message ("deployment instruction created") describes the
// attempt, not the failure — the real reason lives in the logs.
func reconcileAlertLine(page *FeaturePage) g.Node {
	switch page.Status {
	case "FAILED":
		return alertLine("error", "⚠", "Last deploy failed", lastDeployAge(page), logsLink(page, "View logs"))
	case "SENT", "INSTALLING":
		return alertLine("pending", "⟳", "Deploying"+targetVersionSuffix(page)+"…", "", logsLink(page, "View logs"))
	case "PENDING":
		return alertLine("pending", "⟳", "Deploy pending", "", logsLink(page, "View logs"))
	case "MISSING-CONFIG", "MISSING-DEPS", "UNHEALTHY", "RENDER-ERROR":
		msg := page.StatusMessage
		if msg == "" {
			msg = "reconcile blocked"
		}
		return alertLine("warning", "⚠", msg, "", nil)
	default:
		return nil
	}
}

func alertLine(kind, icon, text, age string, logs g.Node) g.Node {
	nodes := []g.Node{
		h.Span(h.Class("reconcile-alert-icon"), g.Text(icon)),
		h.Span(g.Text(text)),
	}
	if age != "" {
		nodes = append(nodes, h.Span(h.Class("text-muted"), g.Text("· "+age)))
	}
	if logs != nil {
		nodes = append(nodes, logs)
	}
	return h.Div(h.Class("reconcile-alert reconcile-alert-"+kind), g.Group(nodes))
}

// isConverged reports whether Fasit has nothing outstanding to do, so the
// reconcile alert line can stay hidden.
func isConverged(page *FeaturePage) bool {
	switch page.Status {
	case "DEPLOYED", "DISABLED", "UNKNOWN", "":
		return true
	default:
		return false
	}
}

func logsLink(page *FeaturePage, label string) g.Node {
	if page.FeatureLog == nil || len(page.FeatureLog.CurrentLog) == 0 {
		return nil
	}
	return h.Button(h.Type("button"), h.Class("btn-link"), g.Attr("popovertarget", "deploy-logs"), g.Text(label))
}

func lastDeployAge(page *FeaturePage) string {
	if len(page.RecentDeployHistory) == 0 {
		return ""
	}
	return view.RelativeTime(page.RecentDeployHistory[0].LastModified)
}

func targetVersionSuffix(page *FeaturePage) string {
	if page.WinningAssignment != nil && page.WinningAssignment.Feature != nil && page.WinningAssignment.Feature.Version != "" {
		return " " + page.WinningAssignment.Feature.Version
	}
	return ""
}

func deployLogsPopover(page *FeaturePage) g.Node {
	if page.FeatureLog == nil || len(page.FeatureLog.CurrentLog) == 0 {
		return nil
	}
	return components.Popover("deploy-logs", "popover-wide", "Logs",
		h.Div(h.Class("popover-scroll"), logBlock(page.FeatureLog.CurrentLog)),
	)
}

// deployHistoryPopover shows the full deploy history for this feature×environment
// in a kebab-toggled popover. Each entry expands to reveal that deploy's Helm
// logs.
func deployHistoryPopover(page *FeaturePage) g.Node {
	if len(page.RecentDeployHistory) == 0 {
		return nil
	}
	items := make([]g.Node, len(page.RecentDeployHistory))
	expandIndex := 0
	if page.ExpandedLogID != "" {
		expandIndex = -1
		for i, di := range page.RecentDeployHistory {
			if di.ID.String() == page.ExpandedLogID {
				expandIndex = i
				break
			}
		}
		if expandIndex == -1 {
			expandIndex = 0
		}
	}
	for i, di := range page.RecentDeployHistory {
		lines := page.DeployLogs[di.ID]
		var body g.Node
		if len(lines) > 0 {
			body = h.Div(h.Class("popover-scroll"), logBlock(lines))
		} else {
			body = h.P(h.Class("text-muted deploy-history-empty"), g.Text("No Helm logs for this deploy."))
		}
		attrs := []g.Node{h.Class("deploy-history-item")}
		if i == expandIndex {
			attrs = append(attrs, g.Attr("open"))
		}
		items[i] = h.Details(append(attrs,
			h.Summary(h.Class("deploy-history-summary"),
				h.Span(h.Class("deploy-version"), g.Text(di.FeatureVersion)),
				components.Status(strings.ToUpper(string(di.Status))),
				h.Span(h.Class("text-muted deploy-history-when"), h.Title(view.FormatTime(di.Created)), g.Text(view.RelativeTime(di.Created))),
				h.Span(h.Class("deploy-log-toggle"), g.Text("Helm logs")),
			),
			body,
		)...)
	}
	return components.Popover("deploy-history", "popover-wide", "Deploy history",
		h.Div(h.Class("deploy-history-list"), g.Group(items)),
	)
}

func reconcileHeaderStatus(page *FeaturePage) g.Node {
	statusClass := "status-success"
	statusText := "Reconcile enabled"
	title := statusText
	if !page.Feature.Enabled {
		statusClass = "status-error"
		statusText = "Reconcile disabled"
		reason := page.Feature.DisableReason
		if reason == "" {
			reason = "disabled before we started requiring reason"
		}
		title = statusText + ": " + reason
	}

	return h.Span(h.Class("reconcile-header-status "+statusClass), h.Title(title),
		h.Span(h.Class("reconcile-header-icon"), g.Text("●")),
		h.Span(g.Text(statusText)),
	)
}

// decisionHistoryPopover renders the reconciler decision history for this
// feature×environment inside a kebab-toggled popover. A decision_log row exists
// only when the decision changed, so it surfaces skip/failure reasons
// (missing-deps, disabled, ...) that never produce a deploy and are therefore
// invisible in the deploy history.
func decisionHistoryPopover(page *FeaturePage) g.Node {
	if len(page.DecisionHistory) == 0 {
		return nil
	}

	rows := g.Map(page.DecisionHistory, func(d *reconciler.DecisionLogEntry) g.Node {
		msg := d.Message
		if msg == "" {
			msg = "—"
		}
		return h.Tr(
			h.Td(decisionActionBadge(d.Action)),
			h.Td(g.Text(d.FeatureVersion)),
			h.Td(g.Text(msg)),
			h.Td(h.Title(view.FormatTime(d.Created)), g.Text(view.RelativeTime(d.Created))),
		)
	})

	return components.Popover("decision-history", "popover-wide", "Reconciler log",
		h.Div(h.Class("popover-scroll"),
			h.Table(h.Class("table table-compact"),
				h.THead(h.Tr(
					h.Th(g.Text("Action")),
					h.Th(g.Text("Version")),
					h.Th(g.Text("Message")),
					h.Th(g.Text("When")),
				)),
				h.TBody(g.Group(rows)),
			),
		),
	)
}

// decisionActionBadge renders a reconciler Action as a coloured pill:
// precondition failures yellow, blocking skips muted, in-progress pending,
// deploy/unchanged success.
func decisionActionBadge(action string) g.Node {
	cls := "text-muted"
	switch action {
	case "missing-deps", "missing-config", "render-error":
		cls = "status-warning"
	case "disabled", "unhealthy":
		cls = "status-disabled"
	case "in-progress":
		cls = "status-pending"
	case "deploy", "unchanged":
		cls = "status-success"
	}
	return h.Span(h.Class(cls), g.Text(action))
}

func overviewTab(page *FeaturePage) g.Node {
	configurable := make([]FeatureConfigItem, 0, len(page.Feature.ConfigItems))
	computed := make([]FeatureConfigItem, 0, len(page.Feature.ConfigItems))
	orphaned := make([]FeatureConfigItem, 0)
	for _, item := range page.Feature.ConfigItems {
		switch {
		case item.IsOrphaned:
			orphaned = append(orphaned, item)
		case item.IsConfigurable:
			configurable = append(configurable, item)
		default:
			computed = append(computed, item)
		}
	}

	return h.Div(h.Class("tab-content-wrapper env-feature-overview"),
		configTable(page, configurable, computed, orphaned),
	)
}

func envActivitySidebar(page *FeaturePage) g.Node {
	if len(page.AuditEntries) == 0 {
		return nil
	}
	title := "Recent activity"
	return h.Aside(h.Class("right-sidebar"),
		components.CardCompact(auditview.ActivityList(auditview.ActivityListParams{
			Title:        title,
			FilterBadge:  page.Tenant.Name + "/" + page.Environment.Name,
			AllHref:      "/auditlog?q=" + url.QueryEscape(page.Feature.Name+" "+page.Tenant.Name+"/"+page.Environment.Name),
			Entries:      page.AuditEntries,
			ResourceNode: envResourceNode,
		})),
	)
}

func envResourceNode(e *audit.Entry) g.Node {
	if e.ObjectType == audit.ObjectTypeConfiguration {
		if i := strings.IndexByte(e.ObjectID, '/'); i > 0 {
			return h.Code(g.Text(e.ObjectID[i+1:]))
		}
	}
	return auditview.ResourceLink(e)
}

func configTable(page *FeaturePage, configurable, computed, orphaned []FeatureConfigItem) g.Node {
	if len(configurable) == 0 && len(computed) == 0 && len(orphaned) == 0 {
		return h.Div(h.H3(g.Text("Configuration")), h.P(h.Class("text-muted"), g.Text("No configuration.")))
	}
	const formID = "config-batch-form"
	basePath := featureBasePathForPage(page)
	header := h.Div(h.Class("config-section-header"),
		h.H3(g.Text("Configuration")),
		g.If(len(configurable) > 0, h.Div(h.Class("config-section-actions"),
			h.Button(h.Type("button"), h.Class("btn-small config-edit-toggle"),
				g.Attr("data-config-edit-toggle", ""),
				g.Raw(`<svg class="btn-icon" viewBox="0 0 16 16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M11.5 2.5l2 2L6 12l-2.5.5.5-2.5 7.5-7.5z"/></svg>`),
				h.Span(g.Text("Edit values"))),
			h.Form(h.ID(formID), h.Method("POST"), h.Action(basePath+"/config/batch"),
				h.Class("config-edit-actions"),
				h.Button(h.Type("submit"), h.Class("btn-small config-save-btn"), g.Text("Save changes")),
				h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("data-config-edit-cancel", ""), g.Text("Cancel")),
			),
		)),
	)

	rows := make([]g.Node, 0, len(configurable)+len(computed))
	for _, item := range configurable {
		rows = append(rows, configurableRow(page, formID, item))
	}
	for _, item := range computed {
		rows = append(rows, computedRow(page, item))
	}

	return h.Div(
		staleValuesSection(page, orphaned),
		h.Div(h.Class("config-editable-section"),
			header,
			h.Table(h.Class("table sortable config-table-env"), g.Attr("data-sort-key", "env-feature-config"),
				h.THead(h.Tr(
					h.Th(g.Text("Configuration Key")),
					h.Th(g.Text("Value")),
					h.Th(g.Text("Source")),
					h.Th(h.Class("config-kebab-col"), g.Attr("data-no-sort", "")),
				)),
				h.TBody(g.Group(rows)),
			),
		),
	)
}

func staleValuesSection(page *FeaturePage, items []FeatureConfigItem) g.Node {
	if len(items) == 0 {
		return nil
	}
	rows := make([]g.Node, 0, len(items))
	for _, item := range items {
		rows = append(rows, staleRow(page, item))
	}
	return h.Div(h.Class("config-stale-section"),
		h.H3(g.Text("\u26a0 Stale values")),
		h.P(h.Class("text-muted"),
			g.Text("These keys are stored for this environment but are not declared in the current Feature.yaml \u2014 typically left over after a value was renamed or removed. They are ignored by the deploy and should be deleted.")),
		h.Table(h.Class("table config-table-env"),
			h.THead(h.Tr(
				h.Th(g.Text("Stale Key")),
				h.Th(g.Text("Value")),
				h.Th(g.Text("Source")),
			)),
			h.TBody(g.Group(rows)),
		),
	)
}

func staleRow(page *FeaturePage, item FeatureConfigItem) g.Node {
	popoverID := "stale-del-" + item.ID
	action := featureBasePathForPage(page) + "/config/delete/" + item.ID
	return h.Tr(h.Class("config-row-stale"),
		h.Td(h.Strong(g.Text(item.Key)), g.Text(" "),
			h.Span(h.Class("config-stale-badge"), g.Text("stale"))),
		components.ConfigValueCell(item),
		h.Td(
			sourceBadge(item),
			h.Button(h.Type("button"), h.Class("config-clear-btn"), g.Attr("popovertarget", popoverID),
				h.Title("Delete value"), g.Text("Delete")),
			components.ConfigDeleteConfirm(popoverID, action, "Delete stale value", "Delete",
				"This removes the stored value for \""+item.Key+"\". It is not declared in the current Feature.yaml.", ""),
		),
	)
}

func configurableRow(page *FeaturePage, formID string, item FeatureConfigItem) g.Node {
	valDef := page.Feature.Values[item.Key]
	warn := valDef.Required && item.Source == string(featurepkg.ConfigSourceHelm) && isEmptyConfigValue(item.Value)
	idForUpdate := ""
	if item.Source == string(featurepkg.ConfigSourceEnv) {
		idForUpdate = item.ID
	}
	return h.Tr(h.ID("config-"+item.Key), g.If(warn, h.Class("config-warning")),
		components.ConfigKeyCell(item),
		components.BulkConfigCell(formID, idForUpdate, item),
		sourceCell(page, item),
		h.Td(h.Class("config-kebab-col"), components.ConfigKebab(page.Feature.Name, item.Key)),
	)
}

func computedRow(page *FeaturePage, item FeatureConfigItem) g.Node {
	var extraKebab []g.Node
	if item.Template != "" {
		testURL := "/template-test?feature=" + url.QueryEscape(page.Feature.Name) +
			"&tenant=" + url.QueryEscape(page.TenantSlug) +
			"&env=" + url.QueryEscape(page.Environment.Name) +
			"&key=" + url.QueryEscape(item.Key)
		extraKebab = append(extraKebab, h.A(h.Href(testURL), h.Class("kebab-item"), g.Text("Test template")))
	}
	return h.Tr(h.ID("config-"+item.Key), h.Class("config-row-computed"),
		components.ConfigKeyCell(item),
		components.ComputedValueCell(item),
		sourceLabelCell(item),
		h.Td(h.Class("config-kebab-col"), components.ConfigKebab(page.Feature.Name, item.Key, extraKebab...)),
	)
}

func sourceLabelCell(item FeatureConfigItem) g.Node {
	return h.Td(sourceBadge(item))
}

// sourceCell renders the source for a configurable row. Environment-level
// overrides are emphasized (to signal the value was set in this view) and get
// an inline Clear control that reverts to the default. The Clear control is
// hidden while in edit mode.
func sourceCell(page *FeaturePage, item FeatureConfigItem) g.Node {
	if item.Source != string(featurepkg.ConfigSourceEnv) {
		return sourceLabelCell(item)
	}
	popoverID := "clear-" + item.ID
	action := featureBasePathForPage(page) + "/config/delete/" + item.ID
	return h.Td(
		sourceBadge(item),
		h.Button(h.Type("button"), h.Class("config-clear-btn"), g.Attr("popovertarget", popoverID),
			h.Title("Clear this value and revert to the default"), g.Text("Clear")),
		components.ConfigDeleteConfirm(popoverID, action, "Clear value", "Clear", "", item.FallbackValue),
	)
}

func sourceBadge(item FeatureConfigItem) g.Node {
	cls, label, title := "source-badge", "values.yaml", "Chart default from values.yaml"
	switch item.Source {
	case string(featurepkg.ConfigSourceEnv):
		cls, label, title = "source-badge source-env-set", "env config", "Set for this environment"
	case string(featurepkg.ConfigSourceGlobal):
		label, title = "global config", "Set for all environments (feature config)"
	default:
		if item.IsComputed {
			if item.IsConfigurable {
				label, title = "computed default", "Default is computed from a template \u2014 set a value to override it"
			} else {
				label, title = "computed", "Computed from a template"
			}
		} else if isEmptyConfigValue(item.Value) {
			label, title = "none", "No value set \u2014 the chart declares this config but provides no default"
		}
	}
	return h.Span(h.Class(cls), h.Title(title), g.Text(label))
}

func logBlock(lines []LogLine) g.Node {
	nodes := make([]g.Node, 0, len(lines)*2)
	for i, line := range lines {
		if i > 0 {
			nodes = append(nodes, g.Text("\n"))
		}
		if line.Timestamp != "" {
			nodes = append(nodes, g.Text(line.Timestamp+" "+line.Message))
		} else {
			nodes = append(nodes, g.Text(line.Message))
		}
	}
	return h.Pre(h.Class("code-block"), g.Group(nodes))
}

func assignmentsTab(page *FeaturePage) g.Node {
	if len(page.Assignments) == 0 {
		return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Assignments")), h.P(g.Text("No assignments target this environment.")))
	}
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Assignments")),
		h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "env-feature-assignments"),
			h.THead(h.Tr(
				h.Th(g.Text("FeatureAssignment")),
				h.Th(g.Text("Version")),
				h.Th(g.Text("Status")),
				h.Th(g.Text("Target")),
				h.Th(g.Text("Created")),
			)),
			h.TBody(g.Group(g.Map(page.Assignments, func(dep EnvAssignmentItem) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href("/assignments/"+dep.ID), g.Text(dep.ID[:8]))),
					h.Td(g.Text(dep.Version)),
					h.Td(components.Status(dep.Status)),
					h.Td(labelPills(dep.TargetLabels)),
					h.Td(g.Text(dep.Created)),
				)
			}))),
		),
	)
}

func labelPills(labels map[string]string) g.Node {
	if len(labels) == 0 {
		return h.Span(h.Class("label-filter-tag"), g.Text("All environments"))
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pills := make([]g.Node, 0, len(keys))
	for _, k := range keys {
		pills = append(pills, h.Span(h.Class("label-filter-tag"), g.Text(k+": "+labels[k])))
	}
	return g.Group(pills)
}

func featureBasePath(r *http.Request) string {
	return featureBasePathValues(chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"))
}

func featureBasePathForPage(page *FeaturePage) string {
	return featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name)
}

func featureBasePathValues(tenant, env, feature string) string {
	return "/features/" + feature + "/envs/" + tenant + "/" + env
}

func redirectOrDefault(r *http.Request, defaultURL string) string {
	if dest := r.FormValue("redirect"); dest != "" && dest[0] == '/' {
		return dest
	}
	return defaultURL
}

func LokiExploreURL(tenant, env, feature string) string {
	ds := tenant + "-logs"
	return "https://monitoring.nais.io/a/grafana-lokiexplore-app/explore/service_name/" + feature + "/logs" +
		"?patterns=%5B%5D&from=now-15m&to=now&var-lineFormat=" +
		"&var-ds=" + ds +
		"&var-filters=k8s_cluster_name%7C%3D%7C" + url.PathEscape(env) +
		"&var-filters=service_name%7C%3D%7C" + url.PathEscape(feature) +
		"&var-fields=&var-levels=&var-metadata=&var-jsonFields=&var-patterns=" +
		"&var-lineFilterV2=&var-lineFilters=&timezone=browser&var-all-fields=" +
		"&displayedFields=%5B%5D&urlColumns=%5B%5D&visualizationType=%22logs%22" +
		"&prettifyLogMessage=false&sortOrder=%22Descending%22&wrapLogMessage=false"
}
