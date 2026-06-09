package features

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/featureenvs"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
	"sigs.k8s.io/yaml"
)

func VersionsTabHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			handleFeatureLoadError(w, r, err)
			return
		}
		data.ActiveTab = "versions"
		data.Versions, err = featurepkg.FeatureVersions(r.Context(), data.CurrentFeature.Name)
		if err != nil {
			http.Error(w, "Failed to load versions", http.StatusInternalServerError)
			return
		}
		data.VersionEnvs = envsByVersion(data.FeatureEnvs)
		renderPage(w, r, layout.Props{Title: data.CurrentFeature.Name + " · Versions", CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func VersionDetailHandler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := loadFeatureData(r)
		if err != nil {
			handleFeatureLoadError(w, r, err)
			return
		}
		version := chi.URLParam(r, "version")
		selected, err := featurepkg.FeatureByNameVersion(r.Context(), data.CurrentFeature.Name, version)
		if err != nil {
			http.Redirect(w, r, "/features/"+data.CurrentFeature.Name+"/versions", http.StatusSeeOther)
			return
		}

		data.ActiveTab = "versions"
		data.IsVersionDetail = true
		data.VersionEnvs = envsByVersion(data.FeatureEnvs)
		data.CurrentFeature = selected

		versionCrumb := breadcrumb.Crumb{Label: version}
		data.Breadcrumbs = []breadcrumb.Crumb{
			breadcrumb.Features(),
			breadcrumb.Feature(selected.Name),
			{Label: "Versions", URL: "/features/" + selected.Name + "/versions"},
			versionCrumb,
		}

		renderPage(w, r, layout.Props{Title: selected.Name + " · " + version, CurrentPage: components.PageFeatures, Content: detailPage(data)})
	}
}

func envsByVersion(envs []featureenvs.Environment) map[string][]featureenvs.Environment {
	ret := map[string][]featureenvs.Environment{}
	for _, env := range envs {
		ret[env.Version] = append(ret[env.Version], env)
	}
	return ret
}

func versionsListContent(data *DetailPage) g.Node {
	featureName := data.CurrentFeature.Name
	if len(data.Versions) == 0 {
		return h.Div(
			h.H2(g.Text("Versions")),
			h.P(h.Class("text-muted"), g.Text("No versions found for this feature.")),
		)
	}
	return h.Div(
		h.H2(g.Text("Versions")),
		h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "feature-versions"),
			h.THead(h.Tr(
				h.Th(g.Text("Version")),
				h.Th(g.Text("Description")),
				h.Th(g.Text("Instances")),
				h.Th(g.Text("Last updated")),
			)),
			h.TBody(g.Group(g.Map(data.Versions, func(v featurepkg.FeatureVersion) g.Node {
				instances := len(data.VersionEnvs[v.Version])
				rowAttrs := []g.Node{}
				if instances == 0 {
					rowAttrs = append(rowAttrs, h.Class("version-inactive"))
				}
				rowAttrs = append(rowAttrs,
					h.Td(h.Span(h.Class("version-cell"),
						h.A(h.Href("/features/"+featureName+"/versions/"+v.Version), g.Text(v.Version)),
						activePill(instances),
					)),
					h.Td(h.Class("text-muted"), g.Text(v.Description)),
					h.Td(g.Text(strconv.Itoa(instances))),
					lastDeployedCell(v.LastUpdated, "text-muted"),
				)
				return h.Tr(rowAttrs...)
			}))),
		),
	)
}

func activePill(instances int) g.Node {
	if instances == 0 {
		return nil
	}
	return h.Span(h.Class("active-pill"), g.Text("Active"))
}

func versionDetailContent(data *DetailPage) g.Node {
	feat := data.CurrentFeature
	envs := data.VersionEnvs[feat.Version]

	return g.Group([]g.Node{
		versionMetadata(feat),
		versionValues(feat),
		versionRawValues(feat),
		versionInstances(feat, envs),
	})
}

const defaultDeployTimeout = 5 * time.Minute

func versionMetadata(feat *featurepkg.Feature) g.Node {
	timeout := "Default (" + defaultDeployTimeout.String() + ")"
	if feat.Timeout.Seconds() > 10 {
		timeout = feat.Timeout.String()
	}
	source := g.Node(g.Text("—"))
	if feat.Source != "" {
		source = h.A(h.Href(feat.Source), h.Target("_blank"), h.Rel("noopener"), g.Text(feat.Source))
	}

	deps := g.Node(h.Span(h.Class("text-muted"), g.Text("None")))
	if len(feat.Dependencies) > 0 {
		deps = yamlInline(feat.Dependencies)
	}

	rows := []g.Node{
		metaRow("Description", g.Text(feat.Description)),
		metaRow("Chart", g.Text(feat.Chart)),
		metaRow("Source", source),
		metaRow("Installation timeout", g.Text(timeout)),
	}

	rows = append(rows, h.Tr(
		h.Td(h.Class("th-like"), g.Text("Dependencies")),
		h.Td(h.Class("deps-value"), deps),
	))

	return h.Table(h.Class("table meta-table table-compact version-meta"),
		h.TBody(g.Group(rows)),
	)
}

func metaRow(label string, value g.Node) g.Node {
	return h.Tr(
		h.Td(h.Class("th-like"), g.Text(label)),
		h.Td(value),
	)
}

func versionInstances(feat *featurepkg.Feature, envs []featureenvs.Environment) g.Node {
	if len(envs) == 0 {
		return h.Div(
			h.H2(g.Text("Instances")),
			h.P(h.Class("text-muted"), g.Text("This version is not active in any environment.")),
		)
	}

	sorted := make([]featureenvs.Environment, len(envs))
	copy(sorted, envs)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].TenantName == sorted[j].TenantName {
			return sorted[i].EnvironmentName < sorted[j].EnvironmentName
		}
		return sorted[i].TenantName < sorted[j].TenantName
	})

	ids := make([]string, len(sorted))
	for i, env := range sorted {
		ids[i] = env.AssignmentID
	}
	emphasis := components.ColumnConsensus(ids)

	rows := make([]g.Node, len(sorted))
	for i, env := range sorted {
		rows[i] = h.Tr(
			h.Td(g.Text(env.TenantName)),
			h.Td(h.A(h.Href("/features/"+feat.Name+"/envs/"+env.TenantSlug+"/"+env.EnvironmentName), g.Text(env.EnvironmentName))),
			h.Td(components.Status(env.Status)),
			h.Td(assignmentIDCell(env.AssignmentID, emphasis[i])),
		)
	}

	return h.Div(
		h.H2(g.Text("Instances")),
		h.Table(h.Class("table sortable"), g.Attr("data-sort-key", "feature-version-instances"),
			h.THead(h.Tr(
				h.Th(g.Text("Tenant")),
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Status")),
				h.Th(g.Text("Assignment")),
			)),
			h.TBody(g.Group(rows)),
		),
	)
}

func assignmentIDCell(assignmentID string, e components.Emphasis) g.Node {
	if assignmentID == "" {
		return components.ConsensusCell(e, h.Span(h.Class("text-muted"), g.Text("—")))
	}
	short := assignmentID
	if len(short) > 8 {
		short = short[:8]
	}
	return components.ConsensusCell(e, h.A(h.Href("/assignments/"+assignmentID), g.Text(short)))
}

func versionValues(feat *featurepkg.Feature) g.Node {
	if len(feat.Values) == 0 {
		return h.Div(
			h.H2(g.Text("Values")),
			h.P(h.Class("text-muted"), g.Text("This version declares no values.")),
		)
	}
	return collapsibleYAMLBlock("Values", feat.Values)
}

func collapsibleYAMLBlock(title string, v any) g.Node {
	pre := yamlPre(v)
	if pre == nil {
		return nil
	}
	return h.Details(h.Class("version-collapsible"),
		h.Summary(h.H2(g.Text(title))),
		pre,
	)
}

func yamlPre(v any) g.Node {
	return yamlPreClass("code-block yaml-highlight", v)
}

func yamlInline(v any) g.Node {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return nil
	}
	return h.Pre(h.Class("yaml-inline"), g.Text(string(yamlBytes)))
}

func yamlPreClass(class string, v any) g.Node {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return nil
	}
	return h.Pre(h.Class(class), g.Group(highlightYAML(string(yamlBytes))))
}

func highlightYAML(s string) []g.Node {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	nodes := make([]g.Node, 0, len(lines)*2)
	for i, line := range lines {
		if i > 0 {
			nodes = append(nodes, g.Text("\n"))
		}
		nodes = append(nodes, highlightYAMLLine(line)...)
	}
	return nodes
}

func highlightYAMLLine(line string) []g.Node {
	indent := line[:len(line)-len(strings.TrimLeft(line, " "))]
	rest := line[len(indent):]
	nodes := []g.Node{g.Text(indent)}

	if strings.HasPrefix(rest, "- ") {
		nodes = append(nodes, g.Text("- "))
		rest = rest[2:]
	} else if rest == "-" {
		return append(nodes, g.Text("-"))
	}

	if rest == "" {
		return nodes
	}
	if strings.HasPrefix(rest, "#") {
		return append(nodes, yamlSpan("yaml-comment", rest))
	}

	if colon := strings.IndexByte(rest, ':'); colon >= 0 {
		after := rest[colon+1:]
		if after == "" || after[0] == ' ' {
			nodes = append(nodes, yamlSpan("yaml-key", rest[:colon+1]))
			return append(nodes, highlightYAMLValue(after)...)
		}
	}
	return append(nodes, highlightYAMLValue(rest)...)
}

func highlightYAMLValue(v string) []g.Node {
	lead := v[:len(v)-len(strings.TrimLeft(v, " "))]
	val := v[len(lead):]
	nodes := []g.Node{}
	if lead != "" {
		nodes = append(nodes, g.Text(lead))
	}
	if val == "" {
		return nodes
	}
	return append(nodes, yamlSpan(yamlValueClass(val), val))
}

func yamlValueClass(val string) string {
	switch {
	case val == "true" || val == "false" || val == "null" || val == "~":
		return "yaml-bool"
	case val[0] == '"' || val[0] == '\'':
		return "yaml-string"
	default:
		if _, err := strconv.ParseFloat(val, 64); err == nil {
			return "yaml-number"
		}
		return "yaml-string"
	}
}

func yamlSpan(class, text string) g.Node {
	return h.Span(h.Class(class), g.Text(text))
}

func versionRawValues(feat *featurepkg.Feature) g.Node {
	if len(feat.ValuesYAML) == 0 {
		return nil
	}
	return collapsibleYAMLBlock("values.yaml", feat.ValuesYAML)
}
