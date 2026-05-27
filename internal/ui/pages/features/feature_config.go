package features

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/nais/fasit/internal/dbtx"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/components"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func loadGlobalConfigItems(ctx context.Context, feat *model.Feature) ([]components.ConfigItem, error) {
	configs, err := featurepkg.ConfigGet(ctx, feat.Name)
	if err != nil {
		return nil, err
	}

	declaredKeys := make(map[string]struct{}, len(feat.Values))
	for key := range feat.Values {
		declaredKeys[key] = struct{}{}
	}

	usedKeys := make(map[string]struct{}, len(configs))
	items := make([]components.ConfigItem, 0, len(feat.Values)+len(configs))

	for _, cfg := range configs {
		usedKeys[cfg.Key] = struct{}{}
		item := components.ConfigItem{
			ID:     cfg.ID.String(),
			Key:    cfg.Key,
			Source: string(cfg.Source),
			Value:  components.RawValueForDisplay(cfg.Content),
		}
		if _, ok := declaredKeys[cfg.Key]; !ok {
			item.IsOrphaned = true
		} else {
			populateFromValue(&item, feat.Values[cfg.Key])
			if cfg.Source == model.ConfigSourceGlobal {
				if raw, ok := feat.ValuesYAML[cfg.Key]; ok {
					item.FallbackValue = components.RawValueForDisplay(raw)
				}
			}
		}
		items = append(items, item)
	}

	for key, val := range feat.Values {
		if _, ok := usedKeys[key]; ok {
			continue
		}
		item := components.ConfigItem{
			ID:     uuid.NewSHA1(uuid.Nil, []byte(feat.Name+"|"+key)).String(),
			Key:    key,
			Source: string(model.ConfigSourceHelm),
			Value:  components.RawValueForDisplay(feat.ValuesYAML[key]),
		}
		populateFromValue(&item, val)
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].IsOrphaned != items[j].IsOrphaned {
			return !items[i].IsOrphaned
		}
		return items[i].Key < items[j].Key
	})

	return items, nil
}

func populateFromValue(item *components.ConfigItem, val model.Value) {
	item.DisplayName = val.DisplayName
	item.Description = val.Description
	if val.Config != nil {
		item.IsConfigurable = true
		item.IsSecret = val.Config.Secret
		item.Type = strings.ToUpper(string(val.Config.Type))
	}
	if val.Computed != nil {
		item.IsComputed = true
		item.Template = val.Computed.Template
	}
}

func globalConfigContent(data *DetailPage) g.Node {
	configurable := make([]components.ConfigItem, 0)
	computed := make([]components.ConfigItem, 0)
	orphaned := make([]components.ConfigItem, 0)

	for _, item := range data.ConfigItems {
		if item.IsOrphaned {
			orphaned = append(orphaned, item)
		} else if item.IsComputed {
			computed = append(computed, item)
		} else {
			configurable = append(configurable, item)
		}
	}

	return g.Group([]g.Node{
		globalConfigurableTable(data.CurrentFeature.Name, configurable),
		globalComputedTable(data.CurrentFeature.Name, computed),
		orphanedConfigTable(data.CurrentFeature.Name, orphaned),
	})
}

func globalConfigurableTable(featureName string, items []components.ConfigItem) g.Node {
	if len(items) == 0 {
		return h.Div(h.H2(g.Text("Configuration")), h.P(h.Class("text-muted"), g.Text("No configurable values.")))
	}
	return h.Div(
		h.H2(g.Text("Configuration")),
		h.Table(h.Class("table sortable config-table"), g.Attr("data-sort-key", "feature-global-config"),
			h.THead(h.Tr(
				h.Th(g.Text("Configuration Key")),
				h.Th(h.Class("config-actions-col"), g.Attr("data-no-sort", "")),
				h.Th(g.Text("Value")),
				h.Th(g.Text("Source")),
				h.Th(h.Class("config-kebab-col"), g.Attr("data-no-sort", "")),
			)),
			h.TBody(g.Group(g.Map(items, func(item components.ConfigItem) g.Node {
				return h.Tr(
					components.ConfigKeyCell(item),
					components.ConfigActionsCell(globalConfigActionsCell(featureName, item)),
					components.ConfigValueCell(item),
					globalSourceLabel(item),
					h.Td(h.Class("config-kebab-col"), components.ConfigKebab(featureName, item.Key)),
				)
			}))),
		),
	)
}

func globalComputedTable(featureName string, items []components.ConfigItem) g.Node {
	if len(items) == 0 {
		return nil
	}
	return h.Div(
		h.H2(g.Text("Computed")),
		h.Table(h.Class("table sortable config-table computed-template-table"), g.Attr("data-sort-key", "feature-global-computed"),
			h.THead(h.Tr(
				h.Th(g.Text("Configuration Key")),
				h.Th(g.Text("Template")),
				h.Th(h.Class("config-kebab-col"), g.Attr("data-no-sort", "")),
			)),
			h.TBody(g.Group(g.Map(items, func(item components.ConfigItem) g.Node {
				return h.Tr(
					components.ConfigKeyCell(item),
					components.TemplateCell(item),
					h.Td(h.Class("config-kebab-col"), components.ConfigKebab(featureName, item.Key)),
				)
			}))),
		),
	)
}

func orphanedConfigTable(featureName string, items []components.ConfigItem) g.Node {
	if len(items) == 0 {
		return nil
	}
	return h.Div(
		h.H2(g.Text("Orphaned")),
		h.P(h.Class("text-muted"), g.Text("These global config values no longer match any key in the feature chart and have no effect.")),
		h.Table(h.Class("table config-table"),
			h.THead(h.Tr(
				h.Th(g.Text("Key")),
				h.Th(h.Class("config-actions-col"), g.Attr("data-no-sort", "")),
				h.Th(g.Text("Value")),
			)),
			h.TBody(g.Group(g.Map(items, func(item components.ConfigItem) g.Node {
				return h.Tr(h.Class("config-orphaned"),
					h.Td(h.Span(h.Class("text-muted"), g.Text(item.Key))),
					components.ConfigActionsCell(globalDeleteButton(featureName, item)),
					h.Td(h.Span(h.Class("text-muted"), g.Text(item.Value))),
				)
			}))),
		),
	)
}

func globalSourceLabel(item components.ConfigItem) g.Node {
	label := "helm value"
	if item.Source == string(model.ConfigSourceGlobal) {
		label = "global config"
	}
	return h.Td(h.Span(h.Class("source-label"), g.Text(label)))
}

func globalConfigActionsCell(featureName string, item components.ConfigItem) g.Node {
	if item.Source == string(model.ConfigSourceGlobal) {
		return g.Group([]g.Node{
			components.ConfigEditPopover(
				"edit-"+item.ID,
				"/features/"+featureName+"/config/"+item.ID,
				"Edit Global Config", "Save",
				item,
				h.Input(h.Type("hidden"), h.Name("type"), h.Value(item.Type)),
			),
			globalDeleteButton(featureName, item),
		})
	}
	return components.ConfigEditPopover(
		"set-"+item.Key,
		"/features/"+featureName+"/config/set",
		"Set Global Config", "Save",
		item,
		h.Input(h.Type("hidden"), h.Name("key"), h.Value(item.Key)),
		h.Input(h.Type("hidden"), h.Name("type"), h.Value(item.Type)),
	)
}

func globalDeleteButton(featureName string, item components.ConfigItem) g.Node {
	if item.Source != string(model.ConfigSourceGlobal) {
		return nil
	}
	return components.ConfigDeletePopover(
		"delete-"+item.ID,
		"/features/"+featureName+"/config/"+item.ID+"/delete",
		fmt.Sprintf("Remove global config for %s?", item.Key),
		item.FallbackValue,
	)
}

// UpdateGlobalConfigHandler handles POST /features/{feature}/config/{id}
func UpdateGlobalConfigHandler() http.HandlerFunc {
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

		value, err := components.ParseConfigValue(r.FormValue("value"), r.FormValue("type"), r.FormValue("mode"))
		if err != nil {
			http.Error(w, "Invalid value: "+err.Error(), http.StatusBadRequest)
			return
		}

		raw, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "Failed to encode value", http.StatusBadRequest)
			return
		}

		if err := dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			_, err := featurepkg.ConfigUpdate(ctx, configID, model.UpdateConfiguration{Value: raw})
			return err
		}); err != nil {
			http.Error(w, "Failed to update: "+err.Error(), http.StatusInternalServerError)
			return
		}

		featureName := chi.URLParam(r, "feature")
		http.Redirect(w, r, "/features/"+featureName+"/config", http.StatusSeeOther)
	}
}

// DeleteGlobalConfigHandler handles POST /features/{feature}/config/{id}/delete
func DeleteGlobalConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configID, err := uuid.Parse(chi.URLParam(r, "id"))
		if err != nil {
			http.Error(w, "Invalid configuration id", http.StatusBadRequest)
			return
		}
		if err := dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			return featurepkg.ConfigDelete(ctx, configID)
		}); err != nil {
			http.Error(w, "Failed to delete: "+err.Error(), http.StatusInternalServerError)
			return
		}

		featureName := chi.URLParam(r, "feature")
		http.Redirect(w, r, "/features/"+featureName+"/config", http.StatusSeeOther)
	}
}

// SetGlobalConfigHandler handles POST /features/{feature}/config/set
func SetGlobalConfigHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Invalid form data", http.StatusBadRequest)
			return
		}

		featureName := chi.URLParam(r, "feature")
		key := r.FormValue("key")
		if key == "" {
			http.Error(w, "Key is required", http.StatusBadRequest)
			return
		}

		feat, err := featurepkg.FeatureByName(r.Context(), featureName)
		if err != nil {
			http.Error(w, "Feature not found", http.StatusNotFound)
			return
		}

		value, err := components.ParseConfigValue(r.FormValue("value"), r.FormValue("type"), r.FormValue("mode"))
		if err != nil {
			http.Error(w, "Invalid value: "+err.Error(), http.StatusBadRequest)
			return
		}

		raw, err := json.Marshal(value)
		if err != nil {
			http.Error(w, "Failed to encode value", http.StatusBadRequest)
			return
		}

		secret := false
		if v, ok := feat.Values[key]; ok && v.Config != nil {
			secret = v.Config.Secret
		}

		if err := dbtx.WithTx(r.Context(), func(ctx context.Context) error {
			_, err := featurepkg.ConfigGlobalCreate(ctx, model.NewConfiguration{
				Feature: featureName,
				Key:     key,
				Value:   raw,
				Secret:  secret,
			})
			return err
		}); err != nil {
			http.Error(w, "Failed to save: "+err.Error(), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/features/"+featureName+"/config", http.StatusSeeOther)
	}
}
