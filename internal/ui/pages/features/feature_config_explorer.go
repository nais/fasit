package features

import (
	"context"
	"net/http"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type configExplorerEnv struct {
	ID         uuid.UUID
	Name       string
	TenantName string
}

type configExplorerKey struct {
	Key         string
	DisplayName string
	Description string
	IsSecret    bool
	IsComputed  bool
}

type configExplorerCell struct {
	Value  string
	Source string // "helm value", "global config", "env config"
}

type configExplorerData struct {
	Envs         []configExplorerEnv
	AllKeys      []configExplorerKey
	SelectedKeys []string
	// Cells[envID][key]
	Cells map[uuid.UUID]map[string]configExplorerCell
}

func parseExplorerKeys(r *http.Request, allKeys []configExplorerKey) []string {
	qkeys := r.URL.Query()["keys"]
	if len(qkeys) == 0 {
		// default: all keys
		result := make([]string, len(allKeys))
		for i, k := range allKeys {
			result[i] = k.Key
		}
		return result
	}
	// validate against known keys
	known := make(map[string]bool, len(allKeys))
	for _, k := range allKeys {
		known[k.Key] = true
	}
	var result []string
	for _, k := range qkeys {
		if known[k] {
			result = append(result, k)
		}
	}
	return result
}

func loadConfigExplorerData(ctx context.Context, feat *model.Feature) (*configExplorerData, error) {
	envs, err := featureEnvironments(ctx, feat)
	if err != nil {
		return nil, err
	}

	globalConfigs, err := featurepkg.ConfigGet(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	globalByKey := make(map[string][]byte, len(globalConfigs))
	for _, gc := range globalConfigs {
		globalByKey[gc.Key] = gc.Content
	}

	envOverrides, err := featurepkg.ConfigEnvListByFeature(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	envOverridesByEnvKey := make(map[uuid.UUID]map[string][]byte)
	for _, ov := range envOverrides {
		if _, ok := envOverridesByEnvKey[ov.EnvironmentID]; !ok {
			envOverridesByEnvKey[ov.EnvironmentID] = make(map[string][]byte)
		}
		envOverridesByEnvKey[ov.EnvironmentID][ov.Key] = ov.Content
	}

	var allKeys []configExplorerKey
	cells := make(map[uuid.UUID]map[string]configExplorerCell)

	for _, env := range envs {
		cells[env.ID] = make(map[string]configExplorerCell)
	}

	for key, val := range feat.Values {
		if val.Config == nil && val.Computed == nil {
			continue
		}
		ek := configExplorerKey{
			Key:      key,
			IsSecret: val.Config != nil && val.Config.Secret,
		}
		if val.DisplayName != "" {
			ek.DisplayName = val.DisplayName
		}
		if val.Description != "" {
			ek.Description = val.Description
		}
		if val.Computed != nil {
			ek.IsComputed = true
		}
		allKeys = append(allKeys, ek)

		helmDefault := components.RawValueForDisplay(feat.ValuesYAML[key])

		for _, env := range envs {
			if envKeys, ok := envOverridesByEnvKey[env.ID]; ok {
				if raw, ok := envKeys[key]; ok {
					cells[env.ID][key] = configExplorerCell{
						Value:  components.RawValueForDisplay(raw),
						Source: "env config",
					}
					continue
				}
			}
			if raw, ok := globalByKey[key]; ok {
				cells[env.ID][key] = configExplorerCell{
					Value:  components.RawValueForDisplay(raw),
					Source: "global config",
				}
				continue
			}
			cells[env.ID][key] = configExplorerCell{
				Value:  helmDefault,
				Source: "helm value",
			}
		}
	}

	sort.Slice(allKeys, func(i, j int) bool {
		return allKeys[i].Key < allKeys[j].Key
	})

	return &configExplorerData{
		Envs:    envs,
		AllKeys: allKeys,
		Cells:   cells,
	}, nil
}

func featureEnvironments(ctx context.Context, feat *model.Feature) ([]configExplorerEnv, error) {
	deployments, err := deployment.ListByFeature(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	deployments = latestDeploymentPerTarget(deployments)

	tenants, err := envpkg.ListTenants(ctx)
	if err != nil {
		return nil, err
	}

	type envInfo struct {
		env        *model.Environment
		tenantName string
		labels     map[string]string
	}
	var allEnvs []envInfo
	for _, tenant := range tenants {
		envs, err := envpkg.List(ctx, tenant.ID)
		if err != nil {
			continue
		}
		for _, env := range envs {
			if !featureTargetsKind(feat.EnvironmentKinds, env.Kind) {
				continue
			}
			labels, err := envpkg.GetLabels(ctx, env.ID)
			if err != nil {
				continue
			}
			allEnvs = append(allEnvs, envInfo{env: env, tenantName: tenant.Name, labels: labels})
		}
	}

	var result []configExplorerEnv
	seen := make(map[uuid.UUID]bool)
	for _, env := range allEnvs {
		for _, dep := range deployments {
			if targetMatchesLabels(dep.TargetLabels, env.labels) && !seen[env.env.ID] {
				result = append(result, configExplorerEnv{
					ID:         env.env.ID,
					Name:       env.env.Name,
					TenantName: env.tenantName,
				})
				seen[env.env.ID] = true
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].TenantName != result[j].TenantName {
			return result[i].TenantName < result[j].TenantName
		}
		return result[i].Name < result[j].Name
	})

	return result, nil
}

func configExplorerContent(featureName string, data *configExplorerData) g.Node {
	if len(data.AllKeys) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No configurable values."))
	}

	selectedSet := make(map[string]bool, len(data.SelectedKeys))
	for _, k := range data.SelectedKeys {
		selectedSet[k] = true
	}

	allSelected := len(data.SelectedKeys) == len(data.AllKeys)

	// Key selector
	keySelector := configExplorerKeySelector(featureName, data.AllKeys, selectedSet, allSelected)

	// Find the selected key objects in order
	var selectedKeys []configExplorerKey
	for _, k := range data.AllKeys {
		if selectedSet[k.Key] {
			selectedKeys = append(selectedKeys, k)
		}
	}

	if len(selectedKeys) == 0 {
		return h.Div(keySelector, h.P(h.Class("text-muted"), g.Text("Select at least one config key to compare.")))
	}

	// Table: rows = envs, columns = selected keys
	headers := []g.Node{h.Th(g.Text("Environment"))}
	for _, k := range selectedKeys {
		label := k.Key
		if k.DisplayName != "" {
			label = k.DisplayName
		}
		th := h.Th(h.Title(k.Key), g.Text(label))
		headers = append(headers, th)
	}

	tableRows := g.Map(data.Envs, func(env configExplorerEnv) g.Node {
		cells := []g.Node{
			h.Td(h.Strong(g.Text(env.TenantName)), g.Text(" / "), g.Text(env.Name)),
		}
		envCells := data.Cells[env.ID]
		for _, k := range selectedKeys {
			cells = append(cells, explorerValueCell(k, envCells[k.Key]))
		}
		return h.Tr(g.Group(cells))
	})

	return h.Div(
		keySelector,
		h.Div(h.Class("config-explorer-wrapper"),
			h.Table(h.Class("table config-explorer"),
				h.THead(h.Tr(g.Group(headers))),
				h.TBody(g.Group(tableRows)),
			),
		),
	)
}

func configExplorerKeySelector(featureName string, allKeys []configExplorerKey, selectedSet map[string]bool, allSelected bool) g.Node {
	baseURL := "/features/" + featureName + "/config-explorer"

	var chips []g.Node
	for _, k := range allKeys {
		label := k.Key
		if k.DisplayName != "" {
			label = k.DisplayName
		}
		selected := selectedSet[k.Key]

		var href string
		if selected {
			// remove this key
			href = buildExplorerURL(baseURL, allKeys, selectedSet, k.Key, false)
		} else {
			// add this key
			href = buildExplorerURL(baseURL, allKeys, selectedSet, k.Key, true)
		}

		cls := "explorer-chip"
		if selected {
			cls += " selected"
		}
		chips = append(chips, h.A(h.Class(cls), h.Href(href), h.Title(k.Key), g.Text(label)))
	}

	// all/none toggle
	var toggleHref, toggleLabel string
	if allSelected {
		toggleHref = baseURL + "?keys=_none"
		toggleLabel = "Deselect all"
	} else {
		toggleHref = baseURL
		toggleLabel = "Select all"
	}

	return h.Div(h.Class("explorer-key-selector"),
		h.Span(h.Class("text-muted text-sm"), g.Text("Compare: ")),
		h.A(h.Class("explorer-chip toggle"), h.Href(toggleHref), g.Text(toggleLabel)),
		g.Group(chips),
	)
}

func buildExplorerURL(baseURL string, allKeys []configExplorerKey, selectedSet map[string]bool, toggleKey string, add bool) string {
	var keys []string
	for _, k := range allKeys {
		sel := selectedSet[k.Key]
		if k.Key == toggleKey {
			sel = add
		}
		if sel {
			keys = append(keys, k.Key)
		}
	}
	if len(keys) == 0 {
		return baseURL + "?keys=_none"
	}
	if len(keys) == len(allKeys) {
		return baseURL
	}
	return baseURL + "?keys=" + strings.Join(keys, "&keys=")
}

func explorerValueCell(key configExplorerKey, cell configExplorerCell) g.Node {
	if key.IsSecret {
		return h.Td(
			h.Span(h.Class("text-muted"), g.Text("••••••••")),
			h.Br(),
			h.Span(h.Class("source-label"), g.Text(cell.Source)),
		)
	}
	return h.Td(
		h.Span(h.Title(cell.Value), g.Text(truncateValue(cell.Value, 40))),
		h.Br(),
		h.Span(h.Class("source-label"), g.Text(cell.Source)),
	)
}

func truncateValue(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
