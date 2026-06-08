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
	"github.com/nais/fasit/internal/ui/uidata"
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

	globalConfigs, err := featurepkg.ConfigGet(ctx, feat.Name)
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

	envOverrides, err := featurepkg.ConfigEnvListAllByFeature(ctx, feat.Name)
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
			rendered, secretTaint, _, rerr := featurepkg.HelmValuesWithSecretTaint(ctx, feat, env.ID)
			if rerr != nil {
				row.Value = "(render error)"
				row.Source = "mapping"
			} else if secretTaint[key] {
				row.Value = "••••••••"
				row.Source = "mapping"
				row.IsSecret = true
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
		return h.Div(h.Class("modal-body"),
			h.H3(g.Text(displayName)),
			h.P(h.Class("text-muted"), g.Text("Not deployed to any environments.")),
		)
	}

	return h.Div(h.Class("modal-body"),
		h.H3(g.Text(displayName)),
		g.If(displayName != key, h.P(h.Class("text-muted text-sm"), h.Code(g.Text(key)))),
		h.Table(h.Class("table table-compact"),
			h.THead(h.Tr(
				h.Th(g.Text("Environment")),
				h.Th(g.Text("Value")),
				h.Th(g.Text("Source")),
			)),
			h.TBody(g.Group(g.Map(rows, func(row compareRow) g.Node {
				valueNode := h.Span(g.Text(truncateValue(row.Value, 60)))
				if row.IsSecret {
					valueNode = h.Span(h.Class("text-muted"), g.Text("••••••••"))
				}
				return h.Tr(
					h.Td(h.A(h.Href(row.EnvHref), h.Strong(g.Text(row.TenantName)), g.Text(" / "), g.Text(row.EnvName))),
					h.Td(valueNode),
					h.Td(h.Span(h.Class("source-label"), g.Text(row.Source))),
				)
			}))),
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

	tenants, err := uidata.ListTenants(ctx)
	if err != nil {
		return nil, err
	}

	type envInfo struct {
		env        *envpkg.Environment
		tenantName string
		labels     map[string]string
	}
	var allEnvs []envInfo
	for _, tenant := range tenants {
		envs, err := envpkg.List(ctx, tenant.ID)
		if err != nil {
			return nil, fmt.Errorf("list environments for tenant %s: %w", tenant.Name, err)
		}
		for _, env := range envs {
			if !featureTargetsKind(feat.EnvironmentKinds, env.Kind) {
				continue
			}
			labels, err := envpkg.GetLabels(ctx, env.ID)
			if err != nil {
				return nil, fmt.Errorf("get labels for environment %s: %w", env.Name, err)
			}
			allEnvs = append(allEnvs, envInfo{env: env, tenantName: tenant.Name, labels: labels})
		}
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

func truncateValue(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
