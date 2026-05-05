package environment

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func FeatureTabHandler(renderPage RenderPage, repo database.Repo, activeTab string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeaturePageData(r.Context(), repo, chi.URLParam(r, "tenant"), chi.URLParam(r, "env"), chi.URLParam(r, "feature"), activeTab)
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name,
			CurrentPage: components.PageTenants,
			Content:     featurePageContent(data),
		})
	}
}

func UpdateConfigHandler(_ database.Repo) http.HandlerFunc {
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

		value, err := parseConfigValue(r.FormValue("value"), r.FormValue("type"))
		if err != nil {
			http.Error(w, "Invalid value format: "+err.Error(), http.StatusBadRequest)
			return
		}

		raw, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "Failed to encode value", http.StatusBadRequest)
			return
		}

		if _, err := featurepkg.ConfigUpdate(r.Context(), configID, model.UpdateConfiguration{Value: raw}); err != nil {
			http.Error(w, "Failed to update configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, featureBasePath(r), http.StatusSeeOther)
	}
}

func DeleteConfigHandler(_ database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid configuration id", http.StatusBadRequest)
			return
		}
		if err := featurepkg.ConfigDelete(r.Context(), configID); err != nil {
			http.Error(w, "Failed to delete configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, featureBasePath(r), http.StatusSeeOther)
	}
}

func ConfigOverrideSubmitHandler(repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		tenant, err := envpkg.GetTenantGetByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := repo.EnvironmentGetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		value, err := parseConfigValue(r.FormValue("value"), r.FormValue("type"))
		if err != nil {
			http.Error(w, "Invalid value format: "+err.Error(), http.StatusBadRequest)
			return
		}
		raw, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "Failed to encode value", http.StatusBadRequest)
			return
		}

		_, err = featurepkg.ConfigCreate(r.Context(), model.NewConfiguration{
			EnvironmentID: &env.ID,
			Feature:       chi.URLParam(r, "feature"),
			Key:           r.FormValue("key"),
			Value:         raw,
		})
		if err != nil {
			http.Error(w, "Failed to create configuration: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, featureBasePath(r), http.StatusSeeOther)
	}
}

func ToggleFeatureStateHandler(repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		tenant, err := envpkg.GetTenantGetByName(r.Context(), chi.URLParam(r, "tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := repo.EnvironmentGetByName(r.Context(), tenant.ID, chi.URLParam(r, "env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		feature, err := featurepkg.FeatureByNameForEnv(r.Context(), chi.URLParam(r, "feature"), env.ID)
		if err != nil {
			http.Error(w, "Failed to get feature: "+err.Error(), http.StatusInternalServerError)
			return
		}
		enabled := r.FormValue("enabled") == "true"
		if _, err := featurepkg.FeatureStatesCreateOrUpdate(r.Context(), env.ID, feature, enabled); err != nil {
			http.Error(w, "Failed to toggle feature state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, featureBasePath(r), http.StatusSeeOther)
	}
}

func featurePageContent(page *FeaturePage) g.Node {
	var tabContent g.Node
	switch page.ActiveTab {
	case "logs":
		tabContent = logsTab(page)
	case "helm":
		tabContent = helmTab(page)
	case "rollouts":
		tabContent = rolloutsTab(page)
	case "deployments":
		tabContent = deploymentsTab(page)
	case "audit":
		tabContent = auditTab()
	case "playground":
		tabContent = playgroundTab(page)
	default:
		tabContent = overviewTab(page)
	}

	return h.Div(h.Class("container"),
		components.EnvironmentSidebar(page.Tenant.Name, page.Environment.Name, page.Feature.Name, page.AllFeatures, page.EnabledFeatures),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(page.Breadcrumbs),
			h.Div(h.Class("card"),
				h.Div(h.Class("card-body"),
					h.P(g.Text("Global feature page: "), h.A(h.Href("/features/"+page.Feature.Name), g.Text(page.Feature.Name))),
					components.TabsNav(page.ActiveTab, envFeatureTabs(page.TenantSlug, page.Environment.Name, page.Feature.Name, page.Feature.HasDeployments)),
					tabContent,
				),
			),
		),
	)
}

func envFeatureTabs(tenant, env, feature string, hasDeployments bool) []components.Tab {
	base := featureBasePathValues(tenant, env, feature)
	tabs := []components.Tab{
		{ID: "overview", Href: base, Label: "Overview"},
		{ID: "logs", Href: base + "/logs", Label: "Logs"},
		{ID: "helm", Href: base + "/helm", Label: "Helm Values"},
	}
	if hasDeployments {
		tabs = append(tabs, components.Tab{ID: "deployments", Href: base + "/deployments", Label: "Deployments"})
	} else {
		tabs = append(tabs, components.Tab{ID: "rollouts", Href: base + "/rollouts", Label: "Rollouts"})
	}
	tabs = append(tabs,
		components.Tab{ID: "playground", Href: base + "/playground", Label: "Playground"},
		components.Tab{ID: "audit", Href: base + "/audit", Label: "Audit"},
	)
	return tabs
}

func overviewTab(page *FeaturePage) g.Node {
	popoverID := "toggle-reconcile"
	action := featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name) + "/toggle-reconcile"
	statusClass, statusText, buttonText, newEnabled := "status-error", "✗ Reconcile disabled", "Enable reconcile", "true"
	if page.Feature.Enabled {
		statusClass, statusText, buttonText, newEnabled = "status-success", "✓ Reconcile enabled", "Disable reconcile", "false"
	}
	return h.Div(h.Class("tab-content-wrapper"),
		h.P(
			h.Span(h.Class(statusClass), g.Text(statusText)), g.Text(" "),
			h.Button(h.Type("button"), h.Class("btn-small"), g.Attr("popovertarget", popoverID), g.Text(buttonText)),
			h.Div(g.Attr("popover", ""), h.ID(popoverID),
				h.H3(g.Text("Confirm reconcile toggle")),
				h.Form(h.Method("POST"), h.Action(action), h.Input(h.Type("hidden"), h.Name("enabled"), h.Value(newEnabled)), h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text(buttonText)))),
			),
		),
		h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Configuration Key")), h.Th(g.Text("Value")))), h.TBody(g.Group(g.Map(page.Feature.ConfigItems, func(item FeatureConfigItem) g.Node {
			return h.Tr(h.Td(configKeyCell(item)), h.Td(configValueCell(page, item)))
		})))),
	)
}

func logsTab(page *FeaturePage) g.Node {
	if page.FeatureLog == nil {
		return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Logs")), h.P(g.Text("No logs available.")))
	}
	content := []g.Node{}
	if len(page.FeatureLog.CurrentLog) > 0 {
		content = append(content, h.H2(g.Textf("Current (%s)", page.FeatureLog.CurrentVersion)), h.P(h.Class("text-muted"), g.Textf("Status: %s · Last modified: %s", page.FeatureLog.CurrentStatus, page.FeatureLog.LastModified)))
		if page.FeatureLog.HelmDiff != nil && page.FeatureLog.HelmDiff.Diff != "" && page.FeatureLog.HelmDiff.Difference != model.HelmValueDifferenceFullMatch {
			content = append(content, h.Details(h.Summary(g.Textf("Helm value changes (%s)", strings.ToLower(strings.ReplaceAll(page.FeatureLog.HelmDiff.Difference.String(), "_", " ")))), h.Pre(h.Class("code-block"), g.Raw(page.FeatureLog.HelmDiff.Diff))))
		}
		content = append(content, logBlock(page.FeatureLog.CurrentLog))
	}
	if len(content) == 0 {
		content = append(content, h.H2(g.Text("Logs")), h.P(g.Text("No logs available.")))
	}
	return h.Div(h.Class("tab-content-wrapper"), g.Group(content))
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
		return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Computed Helm Values")), h.P(g.Text("No helm values available.")))
	}
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Computed Helm Values")), h.Pre(h.Class("code-block"), g.Text(prettyJSON(page.HelmValues))))
}

func playgroundTab(page *FeaturePage) g.Node {
	code := page.PlaygroundCode
	if code == "" {
		code = defaultPlaygroundCode
	}

	action := featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name) + "/playground"

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

func rolloutsTab(page *FeaturePage) g.Node {
	if len(page.Rollouts) == 0 {
		return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Rollout History")), h.P(g.Text("No rollout history available.")))
	}
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Rollout History")), h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Version")), h.Th(g.Text("Status")), h.Th(g.Text("Target")), h.Th(g.Text("Created")), h.Th(g.Text("Completed")))), h.TBody(g.Group(g.Map(page.Rollouts, func(rollout RolloutItem) g.Node {
		return h.Tr(h.Td(rolloutVersionCell(rollout)), h.Td(rolloutStatus(rollout.Status)), h.Td(g.Text(emptyFallback(rollout.Target, "-"))), h.Td(g.Text(rollout.Created)), h.Td(g.Text(emptyFallback(rollout.Completed, "-"))))
	})))))
}

func deploymentsTab(page *FeaturePage) g.Node {
	if len(page.Deployments) == 0 {
		return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Deployments")), h.P(g.Text("No deployments target this environment.")))
	}
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Deployments")),
		h.Table(h.Class("table sortable"),
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
					h.Td(rolloutStatus(dep.Status)),
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

func rolloutVersionCell(r RolloutItem) g.Node {
	return h.A(h.Href("/rollouts/"+r.FeatureName+"/"+r.Version), g.Text(r.Version))
}

func auditTab() g.Node {
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Audit Log")), h.P(g.Text("Audit log for configuration changes will be displayed here.")))
}

func configKeyCell(item FeatureConfigItem) g.Node {
	label := item.Key
	if item.DisplayName != "" {
		label = item.DisplayName
	}
	children := []g.Node{h.Strong(g.Text(label))}
	if item.Description != "" {
		children = append(children, h.Br(), h.Small(h.Class("text-muted"), g.Text(item.Description)))
	}
	return g.Group(children)
}

func configValueCell(page *FeaturePage, item FeatureConfigItem) g.Node {
	if item.IsSecret {
		return h.Span(h.Class("text-muted"), g.Text("••••••••"))
	}
	if item.IsComputed {
		return h.Code(g.Text(item.Template))
	}
	var (
		popoverID   string
		action      string
		title       = "Edit Configuration"
		submitLabel = "Save changes"
		formFields  []g.Node
	)
	if item.Source == string(model.ConfigSourceEnv) {
		popoverID = "edit-" + item.ID
		action = featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name) + "/config/edit/" + item.ID
		formFields = []g.Node{h.Input(h.Type("hidden"), h.Name("type"), h.Value(item.Type))}
	} else {
		popoverID = "override-" + item.Key
		action = featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name) + "/config/override"
		title, submitLabel = "Override Configuration", "Save override"
		formFields = []g.Node{h.Input(h.Type("hidden"), h.Name("key"), h.Value(item.Key)), h.Input(h.Type("hidden"), h.Name("type"), h.Value(item.Type))}
	}
	return h.Div(h.Button(h.Type("button"), h.Class("edit-icon"), g.Attr("popovertarget", popoverID), g.Text("✎")), g.If(item.Source == string(model.ConfigSourceEnv), deleteOverrideButton(page, item)), valueSpan(item), h.Div(g.Attr("popover", ""), h.ID(popoverID), h.H3(g.Text(title)), h.Form(h.Method("POST"), h.Action(action), g.Group(formFields), h.Label(g.Text("Configuration Key")), h.Input(h.Type("text"), h.Value(item.Key), g.Attr("disabled", "")), h.Label(g.Text("Value")), configValueInput(item.Type, item.Value), h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text(submitLabel))))))
}

func valueSpan(item FeatureConfigItem) g.Node {
	if item.Source == string(model.ConfigSourceEnv) {
		return h.Span(h.Class("value-override"), h.Title("source: env override"), g.Text(item.Value))
	}
	return h.Span(h.Class("value-default"), h.Title("source: default"), g.Text(item.Value))
}

func deleteOverrideButton(page *FeaturePage, item FeatureConfigItem) g.Node {
	popoverID := "delete-" + item.ID
	action := featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name) + "/config/delete/" + item.ID
	return g.Group([]g.Node{
		h.Button(h.Type("button"), h.Class("edit-icon"), g.Attr("popovertarget", popoverID), g.Text("✕")),
		h.Div(g.Attr("popover", ""), h.ID(popoverID), h.H3(g.Text("Remove Override")), h.P(g.Textf("Remove the environment override for %s? The global default will be used instead.", item.Key)), h.Form(h.Method("POST"), h.Action(action), h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text("Remove override"))))),
	})
}

func configValueInput(configType, currentValue string) g.Node {
	switch configType {
	case "BOOL":
		return h.Select(h.Name("value"), option("true", currentValue), option("false", currentValue))
	case "INT":
		return h.Input(h.Type("number"), h.Name("value"), h.Value(currentValue))
	case "STRING_ARRAY":
		return h.Textarea(h.Name("value"), g.Text(currentValue))
	default:
		return h.Input(h.Type("text"), h.Name("value"), h.Value(currentValue))
	}
}

func option(value, current string) g.Node {
	attrs := []g.Node{h.Value(value)}
	if value == current {
		attrs = append(attrs, g.Attr("selected", "selected"))
	}
	return h.Option(append(attrs, g.Text(value))...)
}

func emptyFallback(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func rolloutStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED":
		return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED":
		return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING":
		return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
	default:
		return g.Text(status)
	}
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

func featureBasePathValues(tenant, env, feature string) string {
	return "/tenants/" + tenant + "/envs/" + env + "/" + feature
}
