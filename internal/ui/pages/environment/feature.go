package environment

import (
	"bytes"
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
	"github.com/nais/fasit/internal/dbtx"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/pages/auditlog"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeatureContextTabHandler(renderPage RenderPage, activeTab string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		showAll := r.URL.Query().Get("deploys") == "all"
		data, err := loadFeaturePageData(r.Context(), chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), activeTab, r.URL.Query().Get("logs"), showAll)
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       data.Feature.Name + " / " + data.Tenant.Name + " / " + data.Environment.Name,
			CurrentPage: components.PageFeatures,
			Content:     featurePageContent(data),
		})
	}
}

func LegacyFeatureRedirectHandler(suffix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		feature := chi.URLParam(r, "feature")
		tenant := chi.URLParam(r, "tenant")
		env := chi.URLParam(r, "env")
		redirectSuffix := strings.ReplaceAll(suffix, "{id}", chi.URLParam(r, "id"))
		http.Redirect(w, r, "/features/"+feature+"/envs/"+tenant+"/"+env+redirectSuffix, http.StatusSeeOther)
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

// FeatureLogsRedirectHandler redirects /logs to the status tab with the latest
// deploy instruction's logs expanded.
func FeatureLogsRedirectHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		featureName := chi.URLParam(r, "feature")
		tenantSlug := chi.URLParam(r, "tenant")
		envName := chi.URLParam(r, "env")
		basePath := "/features/" + featureName + "/envs/" + tenantSlug + "/" + envName

		tenant, err := envpkg.GetTenantByName(r.Context(), tenantSlug)
		if err != nil {
			http.Redirect(w, r, basePath, http.StatusSeeOther)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, envName)
		if err != nil {
			http.Redirect(w, r, basePath, http.StatusSeeOther)
			return
		}
		di, err := featurepkg.GetLatestDeployInstruction(r.Context(), env.ID, featureName)
		if err != nil || di == nil {
			http.Redirect(w, r, basePath, http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, basePath+"?logs="+di.ID.String(), http.StatusSeeOther)
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
			_, err := featurepkg.ConfigEnvUpdate(ctx, configID, model.UpdateConfiguration{Value: raw})
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

		feat, err := deployment.FeatureForEnvironment(r.Context(), env.ID, featureName)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, deployment.ErrFeatureNotFound) {
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
			_, err := featurepkg.ConfigEnvCreate(ctx, model.NewConfiguration{
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
		feature, err := deployment.FeatureForEnvironment(r.Context(), env.ID, chi.URLParam(r, "feature"))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, deployment.ErrFeatureNotFound) {
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
		http.Redirect(w, r, featureBasePath(r), http.StatusSeeOther)
	}
}

func RedeployHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		tenant, err := envpkg.GetTenantByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get tenant: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		feature, err := deployment.FeatureForEnvironment(r.Context(), env.ID, chi.URLParam(r, "feature"))
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, deployment.ErrFeatureNotFound) {
				status = http.StatusNotFound
			}
			http.Error(w, "Failed to get feature: "+err.Error(), status)
			return
		}
		_, disabled, err := featurepkg.FeatureDisabledAt(r.Context(), env.ID, feature.Name)
		if err != nil {
			http.Error(w, "Failed to get feature state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if disabled {
			http.Error(w, "Cannot redeploy a disabled feature", http.StatusBadRequest)
			return
		}
		if err := deployment.InvalidateLatestDeploy(r.Context(), env.ID, feature.Name); err != nil {
			http.Error(w, "Failed to trigger redeploy: "+err.Error(), http.StatusInternalServerError)
			return
		}

		reconciler.TriggerReconcile()

		http.Redirect(w, r, featureBasePath(r), http.StatusSeeOther)
	}
}

func featurePageContent(page *FeaturePage) g.Node {
	var tabContent g.Node
	switch page.ActiveTab {
	case "helm":
		tabContent = helmTab(page)
	case "deployments":
		tabContent = deploymentsTab(page)
	case "config":
		tabContent = overviewTab(page)
	case "playground":
		tabContent = playgroundTab(page)
	default:
		tabContent = statusTab(page)
	}

	return h.Div(h.Class("container"),
		featurePageSidebar(page),
		components.Breadcrumbs(page.Breadcrumbs),
		h.Main(h.Class("main-content"),
			components.Card(
				h.Div(h.Class("card-header-row"),
					envFeatureTabsNav(page),
					h.Div(h.Class("card-header-actions"),
						reconcileHeaderStatus(page),
						pageKebab(page),
					),
				),
				tabContent,
			),
		),
		envActivitySidebar(page),
	)
}

func featurePageSidebar(page *FeaturePage) g.Node {
	return components.FeatureSidebar(page.Feature.Name, "", page.TenantSlug, page.Environment.Name, page.FeatureEnvs)
}

func envFeatureTabsNav(page *FeaturePage) g.Node {
	base := featureBasePathForPage(page)
	tabs := []components.Tab{
		{ID: "status", Href: base, Label: "Status"},
		{ID: "config", Href: base + "/config", Label: "Config"},
		{ID: "helm", Href: base + "/helm", Label: "Helm Values"},
		{ID: "playground", Href: base + "/playground", Label: "Playground"},
	}
	activeTab := page.ActiveTab
	if activeTab == "" {
		activeTab = "status"
	}
	return components.TabsNav(activeTab, tabs)
}

func pageKebab(page *FeaturePage) g.Node {
	kebabID := "page-kebab"
	items := []g.Node{}

	if page.WinningDeployment != nil {
		items = append(items,
			h.A(h.Href("/deployments/"+page.WinningDeployment.ID.String()), h.Class("kebab-item"), g.Text("View deployment spec")),
		)
	}

	redeployPopoverID := "trigger-redeploy"
	reconcilePopoverID := "toggle-reconcile"

	if page.Feature.Enabled {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", redeployPopoverID), g.Text("Trigger redeploy")),
		)
	}

	if page.Feature.Enabled {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item kebab-item-danger"), g.Attr("popovertarget", reconcilePopoverID), g.Text("Disable reconcile")),
		)
	} else {
		items = append(items,
			h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("popovertarget", reconcilePopoverID), g.Text("Enable reconcile")),
		)
	}

	return h.Div(h.Class("kebab-wrap"),
		h.Button(
			h.Type("button"),
			h.Class("kebab-btn"),
			g.Attr("data-kebab-toggle", kebabID),
			g.Attr("aria-label", "Actions"),
			g.Text("\u22ee"),
		),
		h.Div(h.Class("kebab-menu"), h.ID(kebabID), g.Group(items)),
		redeployPopover(page),
		reconcilePopover(page),
	)
}

func redeployPopover(page *FeaturePage) g.Node {
	if !page.Feature.Enabled {
		return nil
	}
	redeployPopoverID := "trigger-redeploy"
	redeployAction := featureBasePathForPage(page) + "/redeploy"
	return h.Div(g.Attr("popover", ""), h.ID(redeployPopoverID),
		h.H3(g.Text("Confirm redeploy")),
		h.Form(h.Method("POST"), h.Action(redeployAction),
			h.P(g.Textf("Force a fresh deploy of %s in %s?", page.Feature.Name, page.Environment.Name)),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Trigger redeploy")),
				h.Button(h.Type("button"), g.Attr("popovertarget", redeployPopoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func reconcilePopover(page *FeaturePage) g.Node {
	popoverID := "toggle-reconcile"
	action := featureBasePathForPage(page) + "/toggle-reconcile"

	if page.Feature.Enabled {
		return h.Div(g.Attr("popover", ""), h.ID(popoverID),
			h.H3(g.Text("Disable reconcile")),
			h.Form(h.Method("POST"), h.Action(action),
				h.Input(h.Type("hidden"), h.Name("enabled"), h.Value("false")),
				h.P(g.Textf("Disable reconcile for %s in %s? Reconciliation will stop until re-enabled.", page.Feature.Name, page.Environment.Name)),
				h.Label(h.For("reconcile-reason"), g.Text("Reason for disabling reconcile")),
				h.Textarea(h.ID("reconcile-reason"), h.Name("reason"), g.Attr("maxlength", "1000"), g.Attr("required", ""), h.Rows("3")),
				h.Div(h.Class("popover-actions"),
					h.Button(h.Type("submit"), g.Text("Disable reconcile")),
					h.Button(h.Type("button"), g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
				),
			),
		)
	}

	return h.Div(g.Attr("popover", ""), h.ID(popoverID),
		h.H3(g.Text("Enable reconcile")),
		h.Form(h.Method("POST"), h.Action(action),
			h.Input(h.Type("hidden"), h.Name("enabled"), h.Value("true")),
			h.Div(h.Class("popover-actions"),
				h.Button(h.Type("submit"), g.Text("Enable reconcile")),
				h.Button(h.Type("button"), g.Attr("popovertarget", popoverID), g.Attr("popovertargetaction", "hide"), g.Text("Cancel")),
			),
		),
	)
}

func statusTab(page *FeaturePage) g.Node {
	return h.Div(h.Class("tab-content-wrapper env-feature-status"),
		statusReconcileSection(page),
		statusDeploysSection(page),
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

func statusReconcileSection(page *FeaturePage) g.Node {
	if page.Feature.Enabled {
		return nil
	}

	reason := page.Feature.DisableReason
	if reason == "" {
		reason = "disabled before we started requiring reason"
	}

	return h.Section(h.Class("status-section"),
		h.H3(g.Text("Reconciliation")),
		h.Div(h.Class("reconcile-reason"), g.Text(reason)),
	)
}

func statusDeploysSection(page *FeaturePage) g.Node {
	if len(page.RecentDeployHistory) == 0 && page.FeatureLog == nil {
		return h.Section(h.Class("status-section"),
			h.H3(g.Text("Deploys")),
			h.P(h.Class("text-muted"), g.Text("No deploy history.")),
		)
	}

	currentFound := false
	rows := g.Map(page.RecentDeployHistory, func(di *model.DeployInstruction) g.Node {
		isCurrent := !currentFound && page.FeatureLog != nil && di.FeatureVersion == page.FeatureLog.CurrentVersion && string(di.Status) == strings.ToLower(page.FeatureLog.CurrentStatus)
		if isCurrent {
			currentFound = true
		}
		diID := di.ID.String()
		expanded := page.ExpandedLogID == diID
		lines := page.DeployLogsByInstruction[diID]

		var logLink g.Node
		if expanded {
			logLink = h.A(h.Href(featureBasePathForPage(page)), h.Class("btn-link log-toggle"), g.Text("Logs "), h.Span(h.Class("log-arrow"), g.Text("\u25b4")))
		} else {
			logLink = h.A(h.Href(featureBasePathForPage(page)+"?logs="+diID), h.Class("btn-link log-toggle"), g.Text("Logs "), h.Span(h.Class("log-arrow"), g.Text("\u25be")))
		}

		var rowClickURL string
		if expanded {
			rowClickURL = featureBasePathForPage(page)
		} else {
			rowClickURL = featureBasePathForPage(page) + "?logs=" + diID
		}

		rowClass := "deploy-row-clickable"
		if currentFound && !isCurrent {
			rowClass += " deploy-row-superseded"
		}

		nodes := []g.Node{
			h.Tr(
				h.Class(rowClass),
				g.Attr("onclick", "window.location.href='"+rowClickURL+"'"),
				h.Td(
					h.Span(h.Class("deploy-version"),
						g.Text(di.FeatureVersion),
						g.If(isCurrent, h.Span(h.Class("badge-current"), g.Text("current"))),
					),
				),
				h.Td(components.Status(strings.ToUpper(string(di.Status)))),
				h.Td(h.Title(view.FormatTime(di.Created)), g.Text(view.RelativeTime(di.Created))),
				h.Td(h.Title(view.FormatTime(di.LastModified)), g.Text(view.RelativeTime(di.LastModified))),
				h.Td(logLink),
			),
		}
		if expanded {
			var logContent g.Node
			if len(lines) > 0 {
				logContent = logBlock(lines)
			} else {
				logContent = h.P(h.Class("text-muted"), g.Text("No logs available."))
			}
			nodes = append(nodes, h.Tr(h.Class("log-row"),
				h.Td(g.Attr("colspan", "5"), logContent),
			))
		}
		return g.Group(nodes)
	})

	return h.Section(h.Class("status-section"),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Version")),
				h.Th(g.Text("Status")),
				h.Th(g.Text("Created")),
				h.Th(g.Text("Last modified")),
				h.Th(),
			)),
			h.TBody(g.Group(rows)),
		),
		g.If(!page.ShowAllDeploys,
			h.Div(h.Style("text-align: center; padding: 0.75rem 0;"),
				h.A(h.Href(featureBasePathForPage(page)+"?deploys=all"), h.Class("link-muted"), g.Text("Show all \u25be")),
			),
		),
	)
}

func overviewTab(page *FeaturePage) g.Node {
	configurable := make([]FeatureConfigItem, 0, len(page.Feature.ConfigItems))
	computed := make([]FeatureConfigItem, 0, len(page.Feature.ConfigItems))
	for _, item := range page.Feature.ConfigItems {
		if item.IsConfigurable {
			configurable = append(configurable, item)
		} else {
			computed = append(computed, item)
		}
	}

	return h.Div(h.Class("tab-content-wrapper env-feature-overview"),
		configurableTable(page, configurable),
		computedTable(page, computed),
	)
}

func envActivitySidebar(page *FeaturePage) g.Node {
	if len(page.AuditEntries) == 0 {
		return nil
	}
	return h.Aside(h.Class("right-sidebar"),
		components.CardCompact(recentEnvironmentActivity(page)),
	)
}

func recentEnvironmentActivity(page *FeaturePage) g.Node {
	if len(page.AuditEntries) == 0 {
		return nil
	}
	items := make([]g.Node, 0, len(page.AuditEntries))
	for _, entry := range page.AuditEntries {
		description := auditlog.Description(entry)
		items = append(items, h.Li(
			h.Div(h.Class("env-activity-meta"),
				h.Span(
					g.Text(string(entry.Action)),
					g.If(entry.Actor != "", g.Group([]g.Node{g.Text(" by "), h.Span(h.Class("env-activity-actor"), view.ActorNode(entry.Actor))})),
				),
				h.Span(h.Title(view.FormatTime(entry.CreatedAt)), g.Text(view.RelativeTime(entry.CreatedAt))),
			),
			h.Div(h.Class("env-activity-resource"), auditlog.ResourceLink(entry)),
			g.If(description != "", h.Div(h.Class("env-activity-description"), g.Text(description))),
		))
	}
	return h.Section(h.Class("env-activity"),
		h.Div(h.Class("env-activity-header"),
			h.Div(
				h.H3(g.Text("Recent activity")),
				h.Span(h.Class("activity-filter-badge"), g.Text(page.Tenant.Name+"/"+page.Environment.Name)),
			),
			h.A(h.Href("/auditlog?q="+url.QueryEscape(page.Feature.Name+" "+page.Tenant.Name+"/"+page.Environment.Name)), h.Class("link-muted"), g.Text("All →")),
		),
		h.Ul(h.Class("env-activity-list"), g.Group(items)),
	)
}

func configurableTable(page *FeaturePage, items []FeatureConfigItem) g.Node {
	if len(items) == 0 {
		return h.Div(h.H2(g.Text("Configuration")), h.P(h.Class("text-muted"), g.Text("No configurable values.")))
	}
	return h.Div(
		h.H3(g.Text("Configuration")),
		h.Table(h.Class("table sortable config-table"), g.Attr("data-sort-key", "env-feature-config-configurable"),
			h.THead(h.Tr(
				h.Th(g.Text("Configuration Key")),
				h.Th(h.Class("config-actions-col"), g.Attr("data-no-sort", "")),
				h.Th(g.Text("Value")),
				h.Th(g.Text("Source")),
				h.Th(h.Class("config-kebab-col"), g.Attr("data-no-sort", "")),
			)),
			h.TBody(g.Group(g.Map(items, func(item FeatureConfigItem) g.Node {
				valDef := page.Feature.Values[item.Key]
				warn := valDef.Required && item.Source == string(model.ConfigSourceHelm) && isEmptyConfigValue(item.Value)
				return h.Tr(g.If(warn, h.Class("config-warning")),
					components.ConfigKeyCell(item),
					components.ConfigActionsCell(configActionsCell(page, item)),
					components.ConfigValueCell(item),
					sourceLabelCell(item),
					h.Td(h.Class("config-kebab-col"), components.ConfigKebab(page.Feature.Name, item.Key)),
				)
			})))),
	)
}

func computedTable(page *FeaturePage, items []FeatureConfigItem) g.Node {
	if len(items) == 0 {
		return nil
	}
	return h.Div(
		h.H3(g.Text("Computed")),
		h.Table(h.Class("table sortable config-table"), g.Attr("data-sort-key", "env-feature-config-computed"),
			h.THead(h.Tr(
				h.Th(g.Text("Configuration Key")),
				h.Th(h.Class("config-actions-col"), g.Attr("data-no-sort", "")),
				h.Th(g.Text("Value")),
				h.Th(g.Text("Source")),
				h.Th(h.Class("config-kebab-col"), g.Attr("data-no-sort", "")),
			)),
			h.TBody(g.Group(g.Map(items, func(item FeatureConfigItem) g.Node {
				return h.Tr(
					components.ConfigKeyCell(item),
					components.ConfigActionsCell(),
					components.ComputedValueCell(item),
					sourceLabelCell(item),
					h.Td(h.Class("config-kebab-col"), components.ConfigKebab(page.Feature.Name, item.Key)),
				)
			})))),
	)
}

func sourceLabelCell(item FeatureConfigItem) g.Node {
	return h.Td(h.Span(h.Class("source-label"), g.Text(sourceLabel(item))))
}

func sourceLabel(item FeatureConfigItem) string {
	switch item.Source {
	case string(model.ConfigSourceEnv):
		return "env config"
	case string(model.ConfigSourceGlobal):
		return "global config"
	default:
		if item.IsComputed {
			return "mapping"
		}
		return "helm value"
	}
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

func helmTab(page *FeaturePage) g.Node {
	if page.HelmValues == "" {
		body := []g.Node{h.H2(g.Text("Computed Helm Values"))}
		if page.HelmValuesError != "" {
			body = append(body,
				h.P(g.Text("Failed to render helm values:")),
				h.Pre(h.Class("code-block"), g.Text(page.HelmValuesError)),
			)
		} else {
			body = append(body, h.P(g.Text("No helm values available.")))
		}
		return h.Div(h.Class("tab-content-wrapper"), g.Group(body))
	}
	return h.Div(h.Class("tab-content-wrapper"),
		h.Div(h.Class("code-block-header"),
			h.H2(g.Text("Computed Helm Values")),
			h.Button(h.Type("button"), h.Class("copy-btn"), g.Attr("data-copy-target", "helm-values"), g.Text("Copy")),
		),
		h.Pre(h.Class("code-block"), h.ID("helm-values"), g.Text(prettyJSON(page.HelmValues))),
	)
}

func playgroundTab(page *FeaturePage) g.Node {
	code := page.PlaygroundCode
	if code == "" {
		code = defaultPlaygroundCode
	}

	action := featureBasePathForPage(page) + "/playground"

	return h.Div(h.Class("tab-content-wrapper"),
		h.Form(
			h.Method("POST"),
			h.Action(action),
			h.Div(h.Class("playground-controls"),
				h.Label(
					h.Input(h.Type("checkbox"), h.Name("includeUnset")),
					g.Text(" Include unset config"),
				),
				h.Button(h.Type("submit"), g.Text("Generate")),
			),
			h.Div(h.Class("playground-split"),
				h.Div(h.Class("playground-editor"),
					h.Label(h.For("code"), g.Text("Feature.yaml")),
					h.Textarea(h.Name("code"), h.ID("pg-code"),
						g.Text(code),
					),
				),
				h.Div(h.Class("playground-result"),
					playgroundResultNode(page.PlaygroundResult, page.HelmValues),
				),
			),
		),
	)
}

func playgroundResultNode(result *PlaygroundResult, helmValues string) g.Node {
	if result == nil && helmValues == "" {
		return nil
	}
	if result == nil {
		return h.Div(h.Class("playground-output"),
			h.H2(g.Text("values.yaml")),
			h.Pre(h.Class("code-block"), g.Text(prettyJSON(helmValues))),
		)
	}

	var nodes []g.Node

	if len(result.Errors) > 0 {
		errorItems := make([]g.Node, 0, len(result.Errors))
		for _, e := range result.Errors {
			errorItems = append(errorItems, h.Li(g.Text(e)))
		}
		nodes = append(nodes,
			h.Div(h.Class("playground-errors"),
				h.H2(g.Text("Errors")),
				h.Ul(errorItems...),
			),
		)
	}

	merged := result.Result
	if merged != "" && helmValues != "" {
		merged = mergeValuesJSON(merged, helmValues)
	}

	if merged != "" {
		nodes = append(nodes,
			h.Div(h.Class("playground-output"),
				h.H2(g.Text("values.yaml")),
				h.Pre(h.Class("code-block"), g.Text(strings.TrimSpace(merged))),
			),
		)
	}

	return g.Group(nodes)
}

func mergeValuesJSON(playgroundResult, helmValues string) string {
	var resultMap, helmMap map[string]any
	if err := json.Unmarshal([]byte(playgroundResult), &resultMap); err != nil {
		return playgroundResult
	}
	if err := json.Unmarshal([]byte(helmValues), &helmMap); err != nil {
		return playgroundResult
	}
	deepMerge(resultMap, helmMap)
	b, err := json.MarshalIndent(resultMap, "", "  ")
	if err != nil {
		return playgroundResult
	}
	return string(b)
}

func deepMerge(dst, src map[string]any) {
	for k, v := range src {
		if srcMap, ok := v.(map[string]any); ok {
			if dstMap, ok := dst[k].(map[string]any); ok {
				deepMerge(dstMap, srcMap)
				continue
			}
		}
		if _, exists := dst[k]; !exists {
			dst[k] = v
		}
	}
}

func deploymentsTab(page *FeaturePage) g.Node {
	if len(page.Deployments) == 0 {
		return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Deployments")), h.P(g.Text("No deployments target this environment.")))
	}
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Deployments")),
		h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "env-feature-deployments"),
			h.THead(h.Tr(
				h.Th(g.Text("Deployment")),
				h.Th(g.Text("Version")),
				h.Th(g.Text("Status")),
				h.Th(g.Text("Target")),
				h.Th(g.Text("Created")),
			)),
			h.TBody(g.Group(g.Map(page.Deployments, func(dep EnvDeploymentItem) g.Node {
				return h.Tr(
					h.Td(h.A(h.Href("/deployments/"+dep.ID), g.Text(dep.ID[:8]))),
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

func configActionsCell(page *FeaturePage, item FeatureConfigItem) g.Node {
	basePath := featureBasePathForPage(page)
	if item.Source == string(model.ConfigSourceEnv) {
		return g.Group([]g.Node{
			components.ConfigEditPopover(
				"edit-"+item.ID,
				basePath+"/config/edit/"+item.ID,
				"Edit Configuration", "Save changes",
				item,
				h.Input(h.Type("hidden"), h.Name("type"), h.Value(item.Type)),
			),
			deleteOverrideButton(page, item),
		})
	}
	return components.ConfigEditPopover(
		"override-"+item.Key,
		basePath+"/config/override",
		"Override Configuration", "Save override",
		item,
		h.Input(h.Type("hidden"), h.Name("key"), h.Value(item.Key)),
		h.Input(h.Type("hidden"), h.Name("type"), h.Value(item.Type)),
	)
}

func deleteOverrideButton(page *FeaturePage, item FeatureConfigItem) g.Node {
	action := featureBasePathForPage(page) + "/config/delete/" + item.ID
	return components.ConfigDeletePopover(
		"delete-"+item.ID,
		action,
		fmt.Sprintf("Remove the environment override for %s?", item.Key),
		item.FallbackValue,
	)
}

func prettyJSON(s string) string {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s
	}
	return buf.String()
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
