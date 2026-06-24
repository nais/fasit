package environment

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	"github.com/nais/fasit/internal/auth"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/naisdstatus"
	"github.com/nais/fasit/internal/reconciler"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/nais/fasit/internal/ui/view"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

type Environment struct {
	*envpkg.Environment
	Metadata []MetadataItem
}

type MetadataItem struct {
	Key          string
	Value        string
	IsSecret     bool
	ReferencedBy []string
}

type environmentFeatureRow struct {
	Name           string
	Status         string
	StatusMessage  string
	Version        string
	LastSuccessful time.Time
}

type environmentHealth struct {
	ReportedAt time.Time
	HasReport  bool
}

// releaseRow pairs a Helm release reported by naisd with whether it is
// orphaned: installed in the environment with no feature assignment matching
// the environment's labels.
type releaseRow struct {
	*featurepkg.Release
	Orphaned bool
}

const (
	environmentHealthyThreshold = 60 * time.Second
	environmentStaleThreshold   = 5 * time.Minute
)

const (
	environmentTabDetails  = "details"
	environmentTabFeatures = "features"
	environmentTabValues   = "values"
	environmentTabHelm     = "helm"
)

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantSlug := chi.URLParam(r, "tenant")
		envName := chi.URLParam(r, "env")

		tenant, err := envpkg.GetTenantByName(r.Context(), tenantSlug)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		env, err := envpkg.GetByName(r.Context(), tenant.ID, envName)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		featureRows, err := loadEnvironmentFeatureRows(r.Context(), env)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		allTenants, err := uidata.ListTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tenantEnvs, err := envpkg.List(r.Context(), tenant.ID)
		if err != nil {
			http.Error(w, "Failed to load tenant environments: "+err.Error(), http.StatusInternalServerError)
			return
		}
		activeTab := environmentTab(r.URL.Query().Get("tab"))
		labels, err := envpkg.GetLabels(r.Context(), env.ID)
		if err != nil {
			http.Error(w, "Failed to load environment labels: "+err.Error(), http.StatusInternalServerError)
			return
		}

		environment := &Environment{
			Environment: env,
			Metadata:    getEnvironmentMetadata(env),
		}
		envValues, err := envpkg.ListEnvironmentValuesForEnvironment(r.Context(), env.ID, true)
		if err != nil {
			http.Error(w, "Failed to load environment values: "+err.Error(), http.StatusInternalServerError)
			return
		}
		valueRefs, err := uidata.GetEnvironmentValueReferences(r.Context(), env.ID)
		if err != nil {
			http.Error(w, "Failed to load environment value references: "+err.Error(), http.StatusInternalServerError)
			return
		}
		releases, err := naisdstatus.ListReleaseStatuses(r.Context(), env.ID)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		releaseRows, err := buildReleaseRows(r.Context(), env.ID, releases)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		health, err := loadEnvironmentHealth(r.Context(), env.ID)
		if err != nil {
			http.Error(w, "Failed to load environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		renderPage(w, r, layout.Props{
			Title:       tenant.Name + " / " + env.Name,
			CurrentPage: components.PageEnvironments,
			Content: page([]breadcrumb.Crumb{
				breadcrumb.Environments(),
				tenantCrumb(tenant.Name, toTenantNavs(allTenants)),
				breadcrumb.EnvironmentWithSwitcher(tenant.Name, env.Name, toEnvironmentNavs(tenantEnvs)),
			}, activeTab, tenant, environment, labels, envValues, valueRefs, gcpProjectIDFromValues(envValues), auth.GetEmail(r.Context()), featureRows, releaseRows, health),
		})
	}
}

// UninstallReleaseHandler asks the environment's naisd agent to uninstall a
// release. It is a deliberate operator action for clearing releases that no
// longer have a matching feature assignment; the reconciler never removes
// releases on its own.
func UninstallReleaseHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantSlug := chi.URLParam(r, "tenant")
		envName := chi.URLParam(r, "env")
		releaseName := chi.URLParam(r, "release")

		tenant, err := envpkg.GetTenantByName(r.Context(), tenantSlug)
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}
		env, err := envpkg.GetByName(r.Context(), tenant.ID, envName)
		if err != nil {
			http.Error(w, "Failed to get environment: "+err.Error(), http.StatusInternalServerError)
			return
		}

		rec := reconciler.FromContext(r.Context())
		if rec == nil {
			http.Error(w, "Reconciler unavailable", http.StatusInternalServerError)
			return
		}

		switch diid, err := rec.Uninstall(r.Context(), tenant.Name, env.Name, releaseName); {
		case err == nil:
			if err := audit.Create(r.Context(), audit.CreateParams{
				Action:        audit.ActionUninstall,
				ObjectType:    audit.ObjectTypeFeature,
				ObjectID:      releaseName,
				Feature:       releaseName,
				EnvironmentID: &env.ID,
				Metadata:      map[string]string{"diid": diid.String()},
			}); err != nil {
				http.Error(w, "Failed to record audit entry: "+err.Error(), http.StatusInternalServerError)
				return
			}
		case errors.Is(err, reconciler.ErrUninstallProtected):
			http.Error(w, "Release cannot be uninstalled: "+err.Error(), http.StatusBadRequest)
			return
		default:
			http.Error(w, "Failed to uninstall: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/tenants/"+tenantSlug+"/envs/"+envName+"?tab=helm", http.StatusSeeOther)
	}
}

func metadataValue(item MetadataItem) g.Node {
	var value g.Node
	if item.IsSecret {
		value = h.Span(h.Class("text-muted"), g.Text("••••••••"))
	} else {
		value = g.Text(item.Value)
	}
	if item.ReferencedBy == nil {
		return value
	}
	count := len(item.ReferencedBy)
	tooltip := strings.Join(item.ReferencedBy, ", ")
	return g.Group([]g.Node{
		value,
		g.Text(" "),
		h.Span(h.Class("badge"), h.Title(tooltip), g.Textf("%d ref%s", count, plural(count))),
	})
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func loadEnvironmentFeatureRows(ctx context.Context, env *envpkg.Environment) ([]environmentFeatureRow, error) {
	features, err := uidata.ListEnvironmentFeatures(ctx, env.ID)
	if err != nil {
		return nil, err
	}

	var statuses map[string]reconciler.EnvironmentFeatureStatus
	if env.Reconcile {
		statuses, err = reconciler.FeatureStatusesForEnvironment(ctx, env.ID)
		if err != nil {
			return nil, err
		}
	}
	latest, err := featurepkg.LatestDeployInstructionsForEnvironment(ctx, env.ID)
	if err != nil {
		return nil, err
	}
	deployed, err := featurepkg.LatestDeployedForEnvironment(ctx, env.ID)
	if err != nil {
		return nil, err
	}

	rows := make([]environmentFeatureRow, 0, len(features))
	for _, feature := range features {
		row := environmentFeatureRow{Name: feature.Name, Status: "UNKNOWN"}
		if !env.Reconcile || feature.FeatureDisabled {
			row.Status = "DISABLED"
		} else if s, ok := statuses[feature.Name]; ok && s.Status != "" {
			row.Status = reconciler.NormalizeStatus(s.Status)
			row.StatusMessage = s.Message
		}
		if di := latest[feature.Name]; di != nil {
			row.Version = di.FeatureVersion
		}
		if di := deployed[feature.Name]; di != nil {
			row.LastSuccessful = di.LastModified
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	return rows, nil
}

func environmentTab(tab string) string {
	switch tab {
	case environmentTabDetails, environmentTabValues, environmentTabFeatures, environmentTabHelm:
		return tab
	default:
		return environmentTabFeatures
	}
}

// buildReleaseRows flags each reported release as orphaned when no feature
// assignment matches the environment. ListForEnvironment already returns the
// label-matched, most-specific assignment per feature, so a release whose name
// is absent from that set has no desired assignment in this environment.
func buildReleaseRows(ctx context.Context, envID uuid.UUID, releases []*featurepkg.Release) ([]releaseRow, error) {
	assignments, err := featureassignment.ListForEnvironment(ctx, envID)
	if err != nil {
		return nil, err
	}
	desired := make(map[string]bool, len(assignments))
	for _, a := range assignments {
		desired[a.Feature.Name] = true
	}
	rows := make([]releaseRow, len(releases))
	for i, rel := range releases {
		rows[i] = releaseRow{Release: rel, Orphaned: !desired[rel.Name]}
	}
	return rows, nil
}

func countOrphans(releases []releaseRow) int {
	n := 0
	for _, r := range releases {
		if r.Orphaned {
			n++
		}
	}
	return n
}

func loadEnvironmentHealth(ctx context.Context, environmentID uuid.UUID) (environmentHealth, error) {
	health, err := naisdstatus.Get(ctx, environmentID)
	if err != nil {
		return environmentHealth{}, err
	}
	if health == nil || health.ReportedAt.Year() < 2000 {
		return environmentHealth{}, nil
	}
	return environmentHealth{ReportedAt: health.ReportedAt, HasReport: true}, nil
}

func page(breadcrumbs []breadcrumb.Crumb, activeTab string, tenant *envpkg.Tenant, environment *Environment, labels map[string]string, envValues []*envpkg.EnvironmentValue, valueRefs map[string][]string, gcpProjectID string, userEmail string, features []environmentFeatureRow, releases []releaseRow, health environmentHealth) g.Node {
	summaryNodes := environmentSummaryNodes(environment, labels, gcpProjectID, userEmail, health)
	orphanCount := countOrphans(releases)
	return h.Div(h.Class("container"),
		environmentSidebar(tenant.Name, environment.Name, activeTab, orphanCount),
		components.Breadcrumbs(breadcrumbs, summaryNodes...),
		h.Main(h.Class("main-content"),
			environmentTabContent(activeTab, tenant, environment, labels, envValues, valueRefs, gcpProjectID, userEmail, features, releases, health),
		),
	)
}

func environmentSidebar(tenantName, environmentName, activeTab string, orphanCount int) g.Node {
	base := "/tenants/" + tenantName + "/envs/" + environmentName
	helmLabel := g.Text("Helm Releases")
	if orphanCount > 0 {
		helmLabel = g.Group([]g.Node{
			g.Text("Helm Releases "),
			h.Span(h.Class("badge status-error"),
				h.Title(fmtOrphans(orphanCount)),
				g.Textf("%d", orphanCount)),
		})
	}
	return h.Aside(h.Class("sidebar feature-sidebar"),
		h.Div(h.Class("feature-sidebar-header"),
			h.H4(g.Text(environmentName)),
		),
		h.Div(h.Class("nav"),
			h.Ul(
				environmentNavItem(base+"?tab=features", g.Text("Features"), activeTab == environmentTabFeatures),
				environmentNavItem(base+"?tab=values", g.Text("Values"), activeTab == environmentTabValues),
				environmentNavItem(base+"?tab=helm", helmLabel, activeTab == environmentTabHelm),
				environmentNavItem(base+"?tab=details", g.Text("Details"), activeTab == environmentTabDetails),
			),
		),
	)
}

func fmtOrphans(n int) string {
	return fmt.Sprintf("%d orphaned release%s installed without a matching feature assignment", n, plural(n))
}

func environmentNavItem(href string, label g.Node, active bool) g.Node {
	attrs := []g.Node{h.Href(href)}
	if active {
		attrs = append(attrs, h.Class("active"))
	}
	return h.Li(h.A(append(attrs, label)...))
}

func environmentSummaryNodes(environment *Environment, labels map[string]string, gcpProjectID string, userEmail string, health environmentHealth) []g.Node {
	var items []g.Node

	// Labels
	if len(labels) > 0 {
		labelPills := make([]g.Node, 0, len(labels))
		for _, k := range sortedKeys(labels) {
			labelPills = append(labelPills, h.Span(h.Class("label-filter-tag"), g.Text(k+": "+labels[k])))
		}
		items = append(items, h.Span(h.Class("env-header-item env-header-labels"), g.Group(labelPills)))
	}

	// GCP project
	if gcpProjectID != "" {
		items = append(items, h.A(
			h.Class("env-header-item env-header-link"),
			h.Href(gcpProjectHref(gcpProjectID, userEmail)),
			h.Target("_blank"),
			g.Attr("rel", "noopener noreferrer"),
			g.Attr("title", "Open GCP project"),
			g.Text(gcpProjectID+" "),
			components.ExternalLinkIcon(),
		))
	}

	// Reconcile status
	var reconcileLabel string
	if environment.Reconcile {
		reconcileLabel = "Reconcile: on"
	} else {
		reconcileLabel = "Reconcile: off"
	}
	items = append(items, h.Span(h.Class("env-header-item"), g.Text(reconcileLabel)))

	// naisd health status
	class, label := naisdHealthBucket(health, time.Now())
	icon := naisdHealthIcon(label)
	items = append(items, h.Span(h.Class("env-header-item status-badge "+class), h.Title(naisdHealthTitle(label)), g.Text(icon+" naisd")))

	return []g.Node{h.Div(h.Class("env-header-actions"), g.Group(items))}
}

func environmentTabContent(activeTab string, tenant *envpkg.Tenant, environment *Environment, labels map[string]string, envValues []*envpkg.EnvironmentValue, valueRefs map[string][]string, gcpProjectID string, userEmail string, features []environmentFeatureRow, releases []releaseRow, health environmentHealth) g.Node {
	switch activeTab {
	case environmentTabValues:
		return environmentValuesCard(envValues, valueRefs)
	case environmentTabFeatures:
		return environmentFeaturesCard(tenant.Name, environment.Name, features)
	case environmentTabHelm:
		return helmReleasesCard(tenant.Name, environment.Name, releases)
	case environmentTabDetails:
		return environmentDetailsCard(environment, labels, gcpProjectID, userEmail, health)
	default:
		return environmentFeaturesCard(tenant.Name, environment.Name, features)
	}
}

func naisdHealthOverviewItem(health environmentHealth) g.Node {
	class, label := naisdHealthBucket(health, time.Now())
	return h.Div(h.Class("environment-health-item "+class),
		h.Div(h.Class("environment-health-icon"), g.Text(naisdHealthIcon(label))),
		h.Div(
			h.Div(h.Class("environment-health-title"), g.Text(naisdHealthTitle(label))),
			g.If(health.HasReport, h.Div(h.Class("environment-health-meta"),
				h.Span(h.Title(view.FormatTime(health.ReportedAt)), g.Text("Reported "+view.RelativeTime(health.ReportedAt))),
			)),
			g.If(!health.HasReport, h.Div(h.Class("environment-health-meta"), g.Text("No health report has been received for this environment."))),
		),
	)
}

func naisdHealthTitle(label string) string {
	if label == "no report" {
		return "No Naisd report"
	}
	return "Naisd is " + label
}

func naisdHealthIcon(label string) string {
	switch label {
	case "healthy":
		return "✓"
	case "stale":
		return "×"
	case "dead":
		return "×"
	default:
		return "?"
	}
}

func naisdHealthBucket(health environmentHealth, now time.Time) (string, string) {
	if !health.HasReport {
		return "status-error", "no report"
	}
	age := now.Sub(health.ReportedAt)
	switch {
	case age < environmentHealthyThreshold:
		return "status-success", "healthy"
	case age < environmentStaleThreshold:
		return "status-error", "stale"
	default:
		return "status-error", "dead"
	}
}

func environmentDetailsCard(environment *Environment, labels map[string]string, gcpProjectID string, userEmail string, health environmentHealth) g.Node {
	return h.Div(h.Class("card"),
		h.Div(h.Class("card-body"),
			h.Div(h.Class("environment-details-list"),
				naisdHealthOverviewItem(health),
				g.Group(g.Map(environment.Metadata, func(item MetadataItem) g.Node {
					return h.Div(h.Class("environment-details-item"),
						h.Div(h.Class("environment-details-key"), g.Text(item.Key)),
						h.Div(h.Class("environment-details-value"), metadataValue(item)),
					)
				})),
				g.If(gcpProjectID != "", h.Div(h.Class("environment-details-item"),
					h.Div(h.Class("environment-details-key"), g.Text("GCP Project")),
					h.Div(h.Class("environment-details-value"),
						g.Text(gcpProjectID),
						g.Text(" "),
						h.A(
							h.Href(gcpProjectHref(gcpProjectID, userEmail)),
							g.Attr("target", "_blank"),
							g.Attr("rel", "noopener noreferrer"),
							g.Attr("title", "Open GCP project "+gcpProjectID),
							components.ExternalLinkIcon(),
						),
					),
				)),
				g.If(len(labels) > 0, h.Div(h.Class("environment-labels-section"),
					h.H3(h.Class("environment-labels-heading"), g.Text("Labels")),
					h.Div(h.Class("environment-labels-group"),
						g.Group(g.Map(sortedKeys(labels), func(k string) g.Node {
							return h.Div(h.Class("environment-details-item"),
								h.Div(h.Class("environment-details-key"), g.Text(k)),
								h.Div(h.Class("environment-details-value"), g.Text(labels[k])),
							)
						})),
					),
				)),
			),
		),
	)
}

func environmentValuesCard(envValues []*envpkg.EnvironmentValue, valueRefs map[string][]string) g.Node {
	if len(envValues) == 0 {
		return g.Group(nil)
	}
	return h.Div(h.Class("card"),
		h.Div(h.Class("card-body"),
			h.H2(h.Class("card-section-heading"), g.Text("Environment values")),
			h.Table(h.Class("table"),
				h.TBody(g.Group(g.Map(envValues, func(val *envpkg.EnvironmentValue) g.Node {
					var valNode g.Node
					if val.Secret {
						valNode = h.Span(h.Class("text-muted"), g.Text("••••••••"))
					} else {
						valNode = g.Text(components.RawValueForDisplay(val.Value))
					}
					if refs := valueRefs[val.Key]; len(refs) > 0 {
						tooltip := strings.Join(refs, ", ")
						valNode = g.Group([]g.Node{
							valNode,
							g.Text(" "),
							h.Span(h.Class("badge"), h.Title(tooltip), g.Textf("%d ref%s", len(refs), plural(len(refs)))),
						})
					}
					return h.Tr(
						h.Td(h.Class("th-like width-md"), g.Text(val.Key)),
						h.Td(valNode),
					)
				}))),
			),
		),
	)
}

func environmentFeaturesCard(tenantName, environmentName string, features []environmentFeatureRow) g.Node {
	return h.Div(h.Class("card"),
		h.Div(h.Class("card-body"),
			h.H2(h.Class("card-section-heading"), g.Text("Features in this environment")),
			g.If(len(features) == 0, h.P(h.Class("text-muted"), g.Text("No features target this environment."))),
			g.If(len(features) > 0, h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "environment-features"),
				h.THead(h.Tr(
					h.Th(g.Text("Feature")),
					h.Th(g.Text("Status")),
					h.Th(g.Text("Version")),
					h.Th(h.Title("When the latest successful deploy instruction completed"), g.Text("Deployed")),
				)),
				h.TBody(g.Group(g.Map(features, func(feature environmentFeatureRow) g.Node {
					statusCell := h.Td(components.Status(feature.Status))
					if feature.StatusMessage != "" {
						statusCell = h.Td(h.Title(feature.StatusMessage), components.Status(feature.Status))
					}
					return h.Tr(
						h.Td(h.A(h.Href("/features/"+feature.Name+"/envs/"+tenantName+"/"+environmentName), g.Text(feature.Name))),
						statusCell,
						h.Td(textOrMuted(feature.Version, "unknown")),
						h.Td(timeOrNever(feature.LastSuccessful)),
					)
				}))),
			)),
		),
	)
}

func helmReleasesCard(tenantName, environmentName string, releases []releaseRow) g.Node {
	sort.Slice(releases, func(i, j int) bool { return releases[i].Name < releases[j].Name })
	orphanCount := countOrphans(releases)
	return h.Div(h.Class("card"),
		h.Div(h.Class("card-body"),
			h.H2(h.Class("card-section-heading"), g.Text("Helm releases")),
			h.P(h.Class("text-muted"), g.Text("Actual state reported by naisd from the environment.")),
			g.If(orphanCount > 0, h.P(h.Class("text-muted"), g.Textf("%d release%s installed without a matching feature assignment.", orphanCount, plural(orphanCount)))),
			g.If(len(releases) == 0, h.P(h.Class("text-muted"), g.Text("No releases reported."))),
			g.If(len(releases) > 0, h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "environment-releases"),
				h.THead(h.Tr(
					h.Th(g.Text("Feature")),
					h.Th(g.Text("Status")),
					h.Th(g.Text("Version")),
					h.Th(g.Text("Last deployed")),
					h.Th(g.Text("Last modified")),
					h.Th(g.Text("Created")),
					h.Th(g.Text("Revision")),
					h.Th(g.Text("")),
				)),
				h.TBody(g.Group(g.Map(releases, func(release releaseRow) g.Node {
					nameCell := h.Td(g.Text(release.Name))
					if release.Orphaned {
						nameCell = h.Td(g.Text(release.Name), g.Text(" "),
							h.Span(h.Class("badge status-error"), h.Title("Installed without a matching feature assignment"), g.Text("orphaned")))
					}
					return h.Tr(
						nameCell,
						h.Td(g.Text(release.Status)),
						h.Td(textOrMuted(release.Version, "unknown")),
						h.Td(timeOrNever(release.LastDeployed)),
						h.Td(timeOrNever(release.LastModified)),
						h.Td(timeOrNever(release.Created)),
						h.Td(g.Textf("%d", release.Revision)),
						h.Td(uninstallCell(tenantName, environmentName, release)),
					)
				}))),
			)),
		),
	)
}

// uninstallCell renders an uninstall button for orphaned releases. Protected
// agent releases are never offered (they back-stop the action server-side too).
func uninstallCell(tenantName, environmentName string, release releaseRow) g.Node {
	if !release.Orphaned || release.Name == "naisd" || release.Name == "fasitd" {
		return g.Text("")
	}
	action := "/tenants/" + tenantName + "/envs/" + environmentName + "/releases/" + release.Name + "/uninstall"
	return h.Form(h.Method("post"), h.Action(action),
		g.Attr("onsubmit", "return confirm('Uninstall "+release.Name+" from "+tenantName+"/"+environmentName+"? This runs helm uninstall in the environment.')"),
		h.Button(h.Type("submit"), h.Class("btn btn-danger btn-sm"), g.Text("Uninstall")),
	)
}

func textOrMuted(value, fallback string) g.Node {
	if value == "" {
		return h.Span(h.Class("text-muted"), g.Text(fallback))
	}
	return g.Text(value)
}

func timeOrNever(t time.Time) g.Node {
	if t.IsZero() {
		return h.Span(h.Class("text-muted"), g.Text("never"))
	}
	return h.Span(h.Title(view.FormatTime(t)), g.Text(view.RelativeTime(t)))
}
