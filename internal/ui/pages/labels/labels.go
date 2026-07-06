package labels

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"sort"
	"strings"

	"github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/layout"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type RenderPage func(http.ResponseWriter, *http.Request, layout.Props)

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

func Handler(renderPage RenderPage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantEnvs, err := environment.ListTenantEnvironments(r.Context(), false)
		if err != nil {
			http.Error(w, "Failed to load environments", http.StatusInternalServerError)
			return
		}

		var rows []envRow
		allLabels := map[string]map[string]bool{}

		for _, te := range tenantEnvs {
			labels := te.Labels
			for k, v := range labels {
				if allLabels[k] == nil {
					allLabels[k] = map[string]bool{}
				}
				allLabels[k][v] = true
			}
			rows = append(rows, envRow{
				Tenant:      te.TenantName,
				Environment: te.Name,
				Labels:      labels,
			})
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

		renderPage(w, r, layout.Props{
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

	return h.Div(
		h.Class("labels-page"),
		h.P(h.Class("labels-desc"), g.Text("Select labels to see which environments a target configuration matches.")),
		h.Div(
			h.Class("labels-grid"),
			h.Div(
				h.Class("labels-main"),
				availableLabelsSection(labelKeys, activeFilters, len(rows)),
			),
			sidebar(activeFilters, filteredRows, len(rows)),
		),
	)
}

func availableLabelsSection(labelKeys []labelKeyInfo, activeFilters map[string]string, totalEnvs int) g.Node {
	tableRows := g.Map(labelKeys, func(info labelKeyInfo) g.Node {
		valueTags := make([]g.Node, 0, len(info.Values))
		for _, v := range info.Values {
			class := "label-value-tag"
			var href string
			if activeFilters[info.Key] == v {
				class += " active"
				href = removeFilterURL(activeFilters, info.Key)
			} else {
				href = filterURL(activeFilters, info.Key, v)
			}
			valueTags = append(valueTags, h.A(
				h.Href(href),
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

	return h.Section(
		h.Class("labels-section"),
		h.Table(
			h.Class("table"),
			h.THead(h.Tr(
				h.Th(g.Text("Label")),
				h.Th(g.Text("Possible Values")),
				h.Th(g.Text("Usage")),
			)),
			h.TBody(g.Group(tableRows)),
		),
	)
}

func sidebar(activeFilters map[string]string, filteredRows []envRow, totalCount int) g.Node {
	jsonStr := targetJSON(activeFilters)

	matchTitle := fmt.Sprintf("Matched Environments (%d", len(filteredRows))
	if len(activeFilters) > 0 {
		matchTitle += fmt.Sprintf(" of %d", totalCount)
	}
	matchTitle += ")"

	return h.Aside(
		h.Class("labels-sidebar"),
		h.Div(
			h.Class("labels-sidebar-card"),
			h.H3(g.Text("Target Configuration")),
			h.Div(
				h.Class("labels-json"),
				h.Pre(g.Text(jsonStr)),
			),
			h.P(h.Class("labels-desc"), targetDescription(activeFilters, len(filteredRows))),
			h.Div(
				h.Class("labels-sidebar-header"),
				h.H4(g.Text(matchTitle)),
				g.If(len(activeFilters) > 0, h.A(h.Href("/labels"), h.Class("btn-small"), g.Text("Clear all"))),
			),
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

	return h.Table(
		h.Class("table"),
		h.THead(h.Tr(
			h.Th(g.Text("Tenant")),
			h.Th(g.Text("Environment")),
			h.Th(g.Text("Labels")),
		)),
		h.TBody(g.Group(tableRows)),
	)
}
