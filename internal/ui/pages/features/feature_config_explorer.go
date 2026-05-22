package features

import (
	"context"
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// configExplorerEnv represents one environment column in the explorer.
type configExplorerEnv struct {
	ID         uuid.UUID
	Name       string
	TenantName string
}

// configExplorerRow represents one config key row across all environments.
type configExplorerRow struct {
	Key         string
	DisplayName string
	Description string
	IsSecret    bool
	IsComputed  bool
	// Values per environment ID. Missing key = helm default applies.
	EnvValues map[uuid.UUID]configExplorerCell
}

type configExplorerCell struct {
	Value  string
	Source string // "helm value", "global config", "env config"
}

func loadConfigExplorerData(ctx context.Context, feat *model.Feature) ([]configExplorerEnv, []configExplorerRow, error) {
	// 1. Find all environments where this feature deploys
	envs, err := featureEnvironments(ctx, feat)
	if err != nil {
		return nil, nil, err
	}

	// 2. Load global configs
	globalConfigs, err := featurepkg.ConfigGet(ctx, feat.Name)
	if err != nil {
		return nil, nil, err
	}
	globalByKey := make(map[string][]byte, len(globalConfigs))
	for _, gc := range globalConfigs {
		globalByKey[gc.Key] = gc.Content
	}

	// 3. Load all env overrides for this feature
	envOverrides, err := featurepkg.ConfigEnvListByFeature(ctx, feat.Name)
	if err != nil {
		return nil, nil, err
	}
	// envOverridesByEnvKey[envID][key] = value
	envOverridesByEnvKey := make(map[uuid.UUID]map[string][]byte)
	for _, ov := range envOverrides {
		if _, ok := envOverridesByEnvKey[ov.EnvironmentID]; !ok {
			envOverridesByEnvKey[ov.EnvironmentID] = make(map[string][]byte)
		}
		envOverridesByEnvKey[ov.EnvironmentID][ov.Key] = ov.Content
	}

	// 4. Build rows for configurable keys
	var rows []configExplorerRow
	for key, val := range feat.Values {
		if val.Config == nil && val.Computed == nil {
			continue
		}
		row := configExplorerRow{
			Key:       key,
			IsSecret:  val.Config != nil && val.Config.Secret,
			EnvValues: make(map[uuid.UUID]configExplorerCell, len(envs)),
		}
		if val.DisplayName != "" {
			row.DisplayName = val.DisplayName
		}
		if val.Description != "" {
			row.Description = val.Description
		}
		if val.Computed != nil {
			row.IsComputed = true
		}

		helmDefault := components.RawValueForDisplay(feat.ValuesYAML[key])

		for _, env := range envs {
			if envKeys, ok := envOverridesByEnvKey[env.ID]; ok {
				if raw, ok := envKeys[key]; ok {
					row.EnvValues[env.ID] = configExplorerCell{
						Value:  components.RawValueForDisplay(raw),
						Source: "env config",
					}
					continue
				}
			}
			if raw, ok := globalByKey[key]; ok {
				row.EnvValues[env.ID] = configExplorerCell{
					Value:  components.RawValueForDisplay(raw),
					Source: "global config",
				}
				continue
			}
			row.EnvValues[env.ID] = configExplorerCell{
				Value:  helmDefault,
				Source: "helm value",
			}
		}

		rows = append(rows, row)
	}

	sort.Slice(rows, func(i, j int) bool {
		return rows[i].Key < rows[j].Key
	})

	return envs, rows, nil
}

// featureEnvironments returns all environments where this feature has a matching deployment.
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

func configExplorerContent(envs []configExplorerEnv, rows []configExplorerRow) g.Node {
	if len(rows) == 0 {
		return h.P(h.Class("text-muted"), g.Text("No configurable values."))
	}

	// Table headers: Key | env1 | env2 | ...
	headers := []g.Node{h.Th(g.Text("Configuration Key"))}
	for _, env := range envs {
		headers = append(headers, h.Th(g.Text(fmt.Sprintf("%s / %s", env.TenantName, env.Name))))
	}

	tableRows := g.Map(rows, func(row configExplorerRow) g.Node {
		cells := []g.Node{configExplorerKeyCell(row)}
		for _, env := range envs {
			cells = append(cells, configExplorerValueCell(row, env.ID))
		}
		return h.Tr(g.Group(cells))
	})

	return h.Div(h.Class("config-explorer-wrapper"),
		h.Table(h.Class("table config-explorer"),
			h.THead(h.Tr(g.Group(headers))),
			h.TBody(g.Group(tableRows)),
		),
	)
}

func configExplorerKeyCell(row configExplorerRow) g.Node {
	label := row.Key
	if row.DisplayName != "" {
		label = row.DisplayName
	}
	children := []g.Node{h.Strong(g.Text(label))}
	if row.Description != "" {
		children = append(children, h.Br(), h.Small(h.Class("text-muted"), g.Text(row.Description)))
	}
	if row.IsComputed {
		children = append(children, h.Br(), h.Small(h.Class("text-muted"), g.Text("(computed)")))
	}
	return h.Td(g.Group(children))
}

func configExplorerValueCell(row configExplorerRow, envID uuid.UUID) g.Node {
	cell, ok := row.EnvValues[envID]
	if !ok {
		return h.Td(h.Class("text-muted"), g.Text("—"))
	}
	if row.IsSecret {
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
