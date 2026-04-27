package environment

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

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
		data, err := loadFeaturePageData(r.Context(), repo, r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature"), activeTab)
		if err != nil {
			http.Error(w, "Failed to load data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		renderPage(w, layout.Props{
			Title:          data.Tenant.Name + " / " + data.Environment.Name + " / " + data.Feature.Name,
			CurrentSection: "tenants",
			Content:        featurePageContent(data),
		})
	}
}

func UpdateConfigHandler(repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		configID, err := uuid.Parse(r.PathValue("id"))
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
		configID, err := uuid.Parse(r.PathValue("id"))
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

		tenant, err := envpkg.GetTenantGetByName(r.Context(), r.PathValue("tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := repo.EnvironmentGetByName(r.Context(), tenant.ID, r.PathValue("env"))
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
			Feature:       r.PathValue("feature"),
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

		tenant, err := envpkg.GetTenantGetByName(r.Context(), r.PathValue("tenant"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := repo.EnvironmentGetByName(r.Context(), tenant.ID, r.PathValue("env"))
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		feature, err := featurepkg.FeatureByNameForEnv(r.Context(), r.PathValue("feature"), env.ID)
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
	case "audit":
		tabContent = auditTab()
	default:
		tabContent = overviewTab(page)
	}

	return h.Div(h.Class("container"),
		components.EnvironmentSidebar(page.Tenant.Name, page.Environment.Name, page.Feature.Name, page.AllFeatures, page.EnabledFeatures),
		h.Main(h.Class("main-content"),
			components.Breadcrumbs(page.Breadcrumbs),
			h.Div(h.Class("card"),
				h.Div(h.Class("card-body"),
					h.P(g.Text("Global feature page: "), h.A(h.Href("/ui/features/"+page.Feature.Name+"/"), g.Text(page.Feature.Name))),
					components.TabsNav(page.ActiveTab, envFeatureTabs()),
					tabContent,
				),
			),
		),
	)
}

func envFeatureTabs() []components.Tab {
	return []components.Tab{{ID: "overview", Href: "./", Label: "Overview"}, {ID: "logs", Href: "./logs", Label: "Logs"}, {ID: "helm", Href: "./helm", Label: "Feature Values"}, {ID: "rollouts", Href: "./rollouts", Label: "Rollouts"}, {ID: "playground", Href: "./playground", Label: "Playground"}, {ID: "audit", Href: "./audit", Label: "Audit"}}
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
		h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Configuration Key")), h.Th(g.Text("Value")))), h.TBody(g.Group(g.Map(page.Feature.ConfigItems, func(item FeatureConfigItem) g.Node { return h.Tr(h.Td(configKeyCell(item)), h.Td(configValueCell(page, item))) })))),
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
			content = append(content, h.Details(h.Summary(g.Textf("Feature value changes (%s)", strings.ToLower(strings.ReplaceAll(page.FeatureLog.HelmDiff.Difference.String(), "_", " ")))), h.Pre(h.Class("code-block"), g.Raw(page.FeatureLog.HelmDiff.Diff))))
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
		if i > 0 { nodes = append(nodes, g.Text("\n")) }
		if line.Timestamp != "" { nodes = append(nodes, g.Text(line.Timestamp+" "+line.Message)) } else { nodes = append(nodes, g.Text(line.Message)) }
	}
	return h.Pre(h.Class("code-block"), g.Group(nodes))
}

func helmTab(page *FeaturePage) g.Node {
	if page.HelmValues == "" { return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Computed Feature Values")), h.P(g.Text("No feature values available."))) }
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Computed Feature Values")), h.Pre(h.Class("code-block"), g.Text(prettyJSON(page.HelmValues))))
}

func rolloutsTab(page *FeaturePage) g.Node {
	if len(page.Rollouts) == 0 { return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Rollout History")), h.P(g.Text("No rollout history available."))) }
	return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Rollout History")), h.Table(h.Class("table sortable"), h.THead(h.Tr(h.Th(g.Text("Version")), h.Th(g.Text("Status")), h.Th(g.Text("Target")), h.Th(g.Text("Created")), h.Th(g.Text("Completed")))), h.TBody(g.Group(g.Map(page.Rollouts, func(rollout RolloutItem) g.Node { return h.Tr(h.Td(rolloutVersionCell(rollout)), h.Td(rolloutStatus(rollout.Status)), h.Td(g.Text(emptyFallback(rollout.Target, "-"))), h.Td(g.Text(rollout.Created)), h.Td(g.Text(emptyFallback(rollout.Completed, "-")))) })))) )
}

func rolloutVersionCell(r RolloutItem) g.Node {
	if r.DeploymentID != "" { return h.A(h.Href("/ui/deployments/"+r.DeploymentID+"/"), g.Text(r.Version)) }
	return h.A(h.Href("/ui/rollouts/"+r.FeatureName+"/"+r.Version+"/"), g.Text(r.Version))
}

func auditTab() g.Node { return h.Div(h.Class("tab-content-wrapper"), h.H2(g.Text("Audit Log")), h.P(g.Text("Audit log for configuration changes will be displayed here."))) }

func configKeyCell(item FeatureConfigItem) g.Node {
	label := item.Key
	if item.DisplayName != "" { label = item.DisplayName }
	children := []g.Node{h.Strong(g.Text(label))}
	if item.Description != "" { children = append(children, h.Br(), h.Small(h.Class("text-muted"), g.Text(item.Description))) }
	return g.Group(children)
}

func configValueCell(page *FeaturePage, item FeatureConfigItem) g.Node {
	if item.IsSecret { return h.Span(h.Class("text-muted"), g.Text("[SECRET]")) }
	if item.IsComputed { return h.Code(g.Text(item.Template)) }
	popoverID, action, title, submitLabel := "", "", "Edit Configuration", "Save changes"
	var formFields []g.Node
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

func valueSpan(item FeatureConfigItem) g.Node { if item.Source == string(model.ConfigSourceEnv) { return h.Span(h.Class("value-override"), h.Title("source: env override"), g.Text(item.Value)) }; return h.Span(h.Class("value-default"), h.Title("source: default"), g.Text(item.Value)) }

func deleteOverrideButton(page *FeaturePage, item FeatureConfigItem) g.Node {
	popoverID := "delete-" + item.ID
	action := featureBasePathValues(page.TenantSlug, page.Environment.Name, page.Feature.Name) + "/config/delete/" + item.ID
	return g.Group([]g.Node{h.Button(h.Type("button"), h.Class("edit-icon"), g.Attr("popovertarget", popoverID), g.Text("✕")), h.Div(g.Attr("popover", ""), h.ID(popoverID), h.H3(g.Text("Remove Override")), h.P(g.Textf("Remove the environment override for %s? The global default will be used instead.", item.Key)), h.Form(h.Method("POST"), h.Action(action), h.Div(h.Class("popover-actions"), h.Button(h.Type("submit"), g.Text("Remove override")))) )})
}

func configValueInput(configType, currentValue string) g.Node {
	switch configType {
	case "BOOL": return h.Select(h.Name("value"), option("true", currentValue), option("false", currentValue))
	case "INT": return h.Input(h.Type("number"), h.Name("value"), h.Value(currentValue))
	case "STRING_ARRAY": return h.Textarea(h.Name("value"), g.Text(currentValue))
	default: return h.Input(h.Type("text"), h.Name("value"), h.Value(currentValue))
	}
}

func option(value, current string) g.Node { attrs := []g.Node{h.Value(value)}; if value == current { attrs = append(attrs, g.Attr("selected", "selected")) }; return h.Option(append(attrs, g.Text(value))...) }
func emptyFallback(value, fallback string) string { if value == "" { return fallback }; return value }

func rolloutStatus(status string) g.Node {
	switch strings.ToUpper(status) {
	case "DEPLOYED": return g.Group([]g.Node{h.Span(h.Class("status-success"), g.Text("✓")), g.Text(" DEPLOYED")})
	case "FAILED": return g.Group([]g.Node{h.Span(h.Class("status-error"), g.Text("✗")), g.Text(" FAILED")})
	case "PENDING": return g.Group([]g.Node{h.Span(h.Class("status-pending"), g.Text("⏳")), g.Text(" PENDING")})
	default: return g.Text(status)
	}
}

func prettyJSON(s string) string { var buf bytes.Buffer; if err := json.Indent(&buf, []byte(s), "", "  "); err != nil { return s }; return buf.String() }
func featureBasePath(r *http.Request) string { return featureBasePathValues(r.PathValue("tenant"), r.PathValue("env"), r.PathValue("feature")) + "/" }
func featureBasePathValues(tenant, env, feature string) string { return "/ui/tenants/" + tenant + "/envs/" + env + "/" + feature }
