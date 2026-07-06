package features

import (
	"context"
	"fmt"
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ConfigCompareHandler returns an HTML fragment showing a single config key's
// resolved value across all environments the feature is deployed to.
func ConfigCompareHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		featureName := chi.URLParam(r, "feature")
		key := chi.URLParam(r, "key")

		ctx := r.Context()
		feat, err := featurepkg.FeatureByName(ctx, featureName)
		if err != nil {
			http.Error(w, "Feature not found", http.StatusNotFound)
			return
		}

		envs, err := featureEnvironments(ctx, feat)
		if err != nil {
			http.Error(w, "Failed to load environments", http.StatusInternalServerError)
			return
		}

		rows, err := compareKeyAcrossEnvs(ctx, feat, key, envs)
		if err != nil {
			http.Error(w, "Failed to compare: "+err.Error(), http.StatusInternalServerError)
			return
		}

		node := configCompareFragment(key, feat, rows)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = node.Render(w)
	}
}

type compareRow struct {
	TenantName string
	EnvName    string
	Value      string
	Source     string
	IsSecret   bool
	EnvHref    string
}

func compareKeyAcrossEnvs(ctx context.Context, feat *featurepkg.Feature, key string, envs []configCompareEnv) ([]compareRow, error) {
	val, hasVal := feat.Values[key]
	isComputed := hasVal && val.Computed != nil
	isSecret := hasVal && val.Config != nil && val.Config.Secret

	globalConfigs, err := featurepkg.GetGlobalConfig(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	var globalValue []byte
	for _, gc := range globalConfigs {
		if gc.Key == key {
			globalValue = gc.Content
			break
		}
	}

	envOverrides, err := featurepkg.ListAllEnvConfigByFeature(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	envOverridesByEnv := make(map[uuid.UUID][]byte)
	for _, ov := range envOverrides {
		if ov.Key == key {
			envOverridesByEnv[ov.EnvironmentID] = ov.Content
		}
	}

	rows := make([]compareRow, 0, len(envs))

	if isComputed {
		for _, env := range envs {
			row := compareRow{
				TenantName: env.TenantName,
				EnvName:    env.Name,
				EnvHref:    "/features/" + feat.Name + "/envs/" + env.TenantName + "/" + env.Name + "/config#config-" + key,
			}
			rendered, rerr := featurepkg.HelmValues(ctx, feat, env.ID)
			if rerr != nil {
				row.Value = "(render error)"
				row.Source = "mapping"
			} else if v, ok := lookupRenderedValue(rendered, key); ok {
				row.Value = v
				row.Source = "mapping"
			} else {
				row.Value = "—"
				row.Source = "mapping"
			}
			rows = append(rows, row)
		}
	} else {
		helmDefault := components.RawValueForDisplay(feat.ValuesYAML[key])
		for _, env := range envs {
			row := compareRow{
				TenantName: env.TenantName,
				EnvName:    env.Name,
				IsSecret:   isSecret,
				EnvHref:    "/features/" + feat.Name + "/envs/" + env.TenantName + "/" + env.Name + "/config#config-" + key,
			}
			if raw, ok := envOverridesByEnv[env.ID]; ok {
				row.Value = components.RawValueForDisplay(raw)
				row.Source = "env config"
			} else if globalValue != nil {
				row.Value = components.RawValueForDisplay(globalValue)
				row.Source = "global config"
			} else {
				row.Value = helmDefault
				row.Source = "helm value"
			}
			rows = append(rows, row)
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TenantName != rows[j].TenantName {
			return rows[i].TenantName < rows[j].TenantName
		}
		return rows[i].EnvName < rows[j].EnvName
	})

	return rows, nil
}

func configCompareFragment(key string, feat *featurepkg.Feature, rows []compareRow) g.Node {
	displayName := key
	if val, ok := feat.Values[key]; ok && val.DisplayName != "" {
		displayName = val.DisplayName
	}

	if len(rows) == 0 {
		return h.Div(
			h.Class("modal-body"),
			h.H3(g.Text(displayName)),
			h.P(h.Class("text-muted"), g.Text("Not deployed to any environments.")),
		)
	}

	return h.Div(
		h.Class("modal-body"),
		h.H3(g.Text(displayName)),
		g.If(displayName != key, h.P(h.Class("text-muted text-sm"), h.Code(g.Text(key)))),
		h.Table(
			h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Value")),
				h.Th(g.Text("Source")),
			)),
			h.TBody(g.Group(configCompareRows(rows))),
		),
	)
}

// configCompareRows builds the table body, applying consensus highlighting to
// the Value and Source columns so the odd one out across environments is easy
// to spot.
func configCompareRows(rows []compareRow) []g.Node {
	valKeys := make([]string, len(rows))
	srcKeys := make([]string, len(rows))
	for i, row := range rows {
		valKeys[i] = row.Value
		srcKeys[i] = row.Source
	}
	valEmph := components.ColumnConsensus(valKeys)
	srcEmph := components.ColumnConsensus(srcKeys)

	body := make([]g.Node, len(rows))
	for i, row := range rows {
		body[i] = h.Tr(
			h.Td(h.A(h.Href(row.EnvHref), h.Strong(g.Text(row.TenantName)), g.Text(" / "), g.Text(row.EnvName))),
			h.Td(components.ConsensusCell(valEmph[i], compareValueNode(i, row.Value))),
			h.Td(components.ConsensusCell(srcEmph[i], h.Span(h.Class("source-label"), g.Text(row.Source)))),
		)
	}
	return body
}

// compareValueNode renders a value cell, truncating long values via CSS and
// always exposing a copy button so the full value is reachable.
func compareValueNode(idx int, value string) g.Node {
	id := fmt.Sprintf("compare-val-%d", idx)
	return h.Span(
		h.Class("compare-value-wrap"),
		h.Span(h.Class("compare-value compare-value-truncated"), h.ID(id), g.Text(value)),
		h.Button(
			h.Type("button"),
			h.Class("copy-btn copy-btn-icon"),
			g.Attr("data-copy-target", id),
			g.Attr("aria-label", "Copy value"),
			h.Title("Copy value"),
			g.Raw(`<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="9" y="9" width="11" height="11" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>`),
		),
	)
}

func lookupRenderedValue(m map[string]any, key string) (string, bool) {
	keys, err := featureutil.SmartDotSplit(key)
	if err != nil || len(keys) == 0 {
		return "", false
	}
	var cur any = m
	for _, k := range keys {
		mm, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		cur, ok = mm[k]
		if !ok {
			return "", false
		}
	}
	switch v := cur.(type) {
	case string:
		return v, true
	case nil:
		return "", true
	default:
		return fmt.Sprintf("%v", v), true
	}
}

type configCompareEnv struct {
	ID         uuid.UUID
	Name       string
	TenantName string
}

func featureEnvironments(ctx context.Context, feat *featurepkg.Feature) ([]configCompareEnv, error) {
	assignments, err := featureassignment.ListByFeature(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	assignments = latestAssignmentPerTarget(assignments)

	tenantEnvs, err := envpkg.ListTenantEnvironments(ctx, false)
	if err != nil {
		return nil, err
	}

	type envInfo struct {
		env        *envpkg.Environment
		tenantName string
		labels     map[string]string
	}
	var allEnvs []envInfo
	for _, te := range tenantEnvs {
		if !featureTargetsKind(feat.EnvironmentKinds, te.Kind) {
			continue
		}
		env := te.Environment
		allEnvs = append(allEnvs, envInfo{env: &env, tenantName: te.TenantName, labels: te.Labels})
	}

	var result []configCompareEnv
	seen := make(map[uuid.UUID]bool)
	for _, env := range allEnvs {
		for _, dep := range assignments {
			if targetMatchesLabels(dep.TargetLabels, env.labels) && !seen[env.env.ID] {
				result = append(result, configCompareEnv{
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
