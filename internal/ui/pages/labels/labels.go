package labels

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strings"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, layout.Props)

type envRow struct {
	Tenant      string
	Environment string
	Labels      map[string]string
}

type labelKeyInfo struct {
	Key    string
	Values []string
	Count  int
}

func Handler(renderPage RenderPage, repo database.Repo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenants, err := environment.GetTenants(r.Context())
		if err != nil {
			http.Error(w, "Failed to load tenants", http.StatusInternalServerError)
			return
		}

		var rows []envRow
		allLabels := map[string]map[string]bool{}

		for _, tenant := range tenants {
			envs, err := repo.EnvironmentsGet(r.Context(), tenant.ID)
			if err != nil {
				continue
			}
			for _, env := range envs {
				labels, err := repo.EnvironmentGetLabels(r.Context(), env.ID)
				if err != nil {
					continue
				}
				labelMap := make(map[string]string, len(labels))
				for _, l := range labels {
					labelMap[l.Key] = l.Value
					if allLabels[l.Key] == nil {
						allLabels[l.Key] = map[string]bool{}
					}
					allLabels[l.Key][l.Value] = true
				}
				rows = append(rows, envRow{
					Tenant:      tenant.Name,
					Environment: env.Name,
					Labels:      labelMap,
				})
			}
		}

		sort.Slice(rows, func(i, j int) bool {
			if rows[i].Tenant != rows[j].Tenant {
				return rows[i].Tenant < rows[j].Tenant
			}
			return rows[i].Environment < rows[j].Environment
		})

		labelKeys := buildLabelKeys(allLabels, rows)

		// Parse active filters from query params: ?label=key:value&label=key2:value2
		activeFilters := parseFilters(r)

		renderPage(w, layout.Props{
			Title:       "Labels",
			CurrentPage: components.PageLabels,
			Content:     page(rows, labelKeys, activeFilters),
		})
	}
}

func parseFilters(r *http.Request) map[string]string {
	filters := map[string]string{}
	for _, lbl := range r.URL.Query()["label"] {
		parts := strings.SplitN(lbl, ":", 2)
		if len(parts) == 2 {
			filters[parts[0]] = parts[1]
		}
	}
	return filters
}

func buildLabelKeys(allLabels map[string]map[string]bool, rows []envRow) []labelKeyInfo {
	keys := make([]string, 0, len(allLabels))
	for k := range allLabels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	infos := make([]labelKeyInfo, 0, len(keys))
	for _, k := range keys {
		values := make([]string, 0, len(allLabels[k]))
		for v := range allLabels[k] {
			values = append(values, v)
		}
		sort.Strings(values)

		count := 0
		for _, row := range rows {
			if _, ok := row.Labels[k]; ok {
				count++
			}
		}

		infos = append(infos, labelKeyInfo{Key: k, Values: values, Count: count})
	}
	return infos
}

func matchesFilters(row envRow, filters map[string]string) bool {
	for k, v := range filters {
		if row.Labels[k] != v {
			return false
		}
	}
	return true
}

func filterURL(current map[string]string, addKey, addValue string) string {
	merged := make(map[string]string, len(current)+1)
	maps.Copy(merged, current)
	merged[addKey] = addValue

	params := make([]string, 0, len(merged))
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		params = append(params, "label="+k+":"+merged[k])
	}
	return "/labels?" + strings.Join(params, "&")
}

func removeFilterURL(current map[string]string, removeKey string) string {
	if len(current) <= 1 {
		return "/labels"
	}
	params := []string{}
	keys := make([]string, 0, len(current))
	for k := range current {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if k != removeKey {
			params = append(params, "label="+k+":"+current[k])
		}
	}
	return "/labels?" + strings.Join(params, "&")
}

func targetJSON(filters map[string]string) string {
	if len(filters) == 0 {
		return "{}"
	}
	b, _ := json.MarshalIndent(filters, "", "  ")
	return string(b)
}

func page(rows []envRow, labelKeys []labelKeyInfo, activeFilters map[string]string) g.Node {
	var filteredRows []envRow
	for _, row := range rows {
		if matchesFilters(row, activeFilters) {
			filteredRows = append(filteredRows, row)
		}
	}

	return h.Div(h.Class("labels-page"),
		h.H1(g.Text("Environment Labels")),
		activeFiltersSection(activeFilters),
		h.Div(h.Class("labels-grid"),
			h.Div(h.Class("labels-main"),
				environmentsSection(filteredRows, activeFilters, len(rows)),
				availableLabelsSection(labelKeys, activeFilters, len(rows)),
			),
			sidebar(activeFilters, filteredRows),
		),
	)
}

func activeFiltersSection(filters map[string]string) g.Node {
	if len(filters) == 0 {
		return nil
	}

	keys := make([]string, 0, len(filters))
	for k := range filters {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tags := make([]g.Node, 0, len(keys))
	for _, k := range keys {
		tags = append(tags, h.A(
			h.Href(removeFilterURL(filters, k)),
			h.Class("label-filter-tag active"),
			g.Text(k+": "+filters[k]+" ✕"),
		))
	}

	return h.Div(h.Class("labels-active-filters"),
		h.Span(h.Class("filter-label"), g.Text("Active filters: ")),
		g.Group(tags),
		h.A(h.Href("/labels"), h.Class("btn-small"), g.Text("Clear all")),
	)
}

func environmentsSection(rows []envRow, activeFilters map[string]string, totalCount int) g.Node {
	countText := fmt.Sprintf("Environments (%d", len(rows))
	if len(activeFilters) > 0 {
		countText += fmt.Sprintf(" of %d", totalCount)
	}
	countText += ")"

	return h.Section(h.Class("labels-section"),
		h.H2(g.Text(countText)),
		h.P(h.Class("labels-desc"), g.Text("All environments with their labels. Click labels to filter.")),
		environmentsTable(rows, activeFilters),
	)
}

func environmentsTable(rows []envRow, activeFilters map[string]string) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("labels-empty"), g.Text("No environments match the selected filters."))
	}

	tableRows := g.Map(rows, func(row envRow) g.Node {
		return h.Tr(
			h.Td(h.Strong(g.Text(row.Tenant))),
			h.Td(g.Text(row.Environment)),
			h.Td(h.Class("labels-cell"), labelTags(row.Labels, activeFilters)),
		)
	})

	return h.Table(h.Class("table sortable"),
		h.THead(h.Tr(
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Labels")),
		)),
		h.TBody(g.Group(tableRows)),
	)
}

func labelTags(labels map[string]string, activeFilters map[string]string) g.Node {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	tags := make([]g.Node, 0, len(keys))
	for _, k := range keys {
		v := labels[k]
		class := "label-filter-tag"
		if activeFilters[k] == v {
			class += " active"
		}
		tags = append(tags, h.A(
			h.Href(filterURL(activeFilters, k, v)),
			h.Class(class),
			g.Text(k+": "+v),
		))
	}
	return g.Group(tags)
}

func availableLabelsSection(labelKeys []labelKeyInfo, activeFilters map[string]string, totalEnvs int) g.Node {
	tableRows := g.Map(labelKeys, func(info labelKeyInfo) g.Node {
		valueTags := make([]g.Node, 0, len(info.Values))
		for _, v := range info.Values {
			class := "label-value-tag"
			if activeFilters[info.Key] == v {
				class += " active"
			}
			valueTags = append(valueTags, h.A(
				h.Href(filterURL(activeFilters, info.Key, v)),
				h.Class(class),
				g.Text(v),
			))
		}

		return h.Tr(
			h.Td(h.Strong(g.Text(info.Key))),
			h.Td(h.Div(h.Class("label-values-list"), g.Group(valueTags))),
			h.Td(g.Text(fmt.Sprintf("%d / %d", info.Count, totalEnvs))),
		)
	})

	return h.Section(h.Class("labels-section"),
		h.H2(g.Text("Available Labels")),
		h.P(h.Class("labels-desc"), g.Text("All label keys and values across all environments. Click values to filter.")),
		h.Table(h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Label")),
				h.Th(g.Text("Possible Values")),
				h.Th(g.Text("Usage")),
			)),
			h.TBody(g.Group(tableRows)),
		),
	)
}

func sidebar(activeFilters map[string]string, filteredRows []envRow) g.Node {
	jsonStr := targetJSON(activeFilters)

	return h.Aside(h.Class("labels-sidebar"),
		h.Div(h.Class("labels-sidebar-card"),
			h.H3(g.Text("Target Configuration")),
			h.Div(h.Class("labels-json"),
				h.Pre(g.Text(jsonStr)),
			),
			h.P(h.Class("labels-desc"), targetDescription(activeFilters, len(filteredRows))),
			h.H4(g.Text(fmt.Sprintf("Matched Environments (%d)", len(filteredRows)))),
			matchedEnvironmentsTable(filteredRows, activeFilters),
		),
	)
}

func targetDescription(filters map[string]string, matchCount int) g.Node {
	if len(filters) == 0 {
		return g.Text("No filters active — targets all environments.")
	}
	return g.Text(fmt.Sprintf("JSON for %d selected label(s), matching %d environment(s).", len(filters), matchCount))
}

func matchedEnvironmentsTable(rows []envRow, activeFilters map[string]string) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("labels-no-match"), g.Text("No environments match the selected labels."))
	}

	tableRows := g.Map(rows, func(row envRow) g.Node {
		keys := make([]string, 0, len(row.Labels))
		for k := range row.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		tags := make([]g.Node, 0, len(keys))
		for _, k := range keys {
			v := row.Labels[k]
			class := "label-filter-tag"
			if activeFilters[k] == v {
				class += " matched"
			}
			tags = append(tags, h.Span(h.Class(class), g.Text(k+": "+v)))
		}

		return h.Tr(
			h.Td(h.Strong(g.Text(row.Tenant))),
			h.Td(g.Text(row.Environment)),
			h.Td(h.Class("labels-cell"), g.Group(tags)),
		)
	})

	return h.Table(h.Class("table"),
		h.THead(h.Tr(
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Labels")),
		)),
		h.TBody(g.Group(tableRows)),
	)
}
