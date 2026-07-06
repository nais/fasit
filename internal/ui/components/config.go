package components

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ConfigItem is the shared data type for config table rows.
type ConfigItem struct {
	ID             string
	Key            string
	DisplayName    string
	Description    string
	Value          string
	Source         string
	Type           string
	IsSecret       bool
	IsComputed     bool
	IsConfigurable bool
	IsOrphaned     bool
	Required       bool
	// HasValue is false when the value is unset (absent or null), distinguishing
	// it from an explicitly set empty string.
	HasValue      bool
	Template      string
	MappedCount   int
	FallbackValue string
}

// ConfigKeyCell renders a <td> with the key, optional display name and description.
func ConfigKeyCell(item ConfigItem) g.Node {
	label := item.Key
	strong := h.Strong(g.Text(label))
	if item.DisplayName != "" {
		label = item.DisplayName
		strong = h.Strong(h.Title(item.Key), g.Text(label))
	}
	children := []g.Node{strong}
	if item.Required {
		children = append(children, g.Text(" "), h.Span(h.Class("config-required-badge"), g.Text("required")))
	}
	if item.Description != "" {
		children = append(children, h.Br(), h.Small(h.Class("text-muted"), g.Text(item.Description)))
	}
	return h.Td(g.Group(children))
}

// ConfigValueCell renders a <td> with the value, masking secrets.
func ConfigValueCell(item ConfigItem) g.Node {
	if item.IsSecret {
		return h.Td(h.Span(h.Class("text-muted"), g.Text("••••••••")))
	}
	return h.Td(ValueDisplay(item.Value, item.Type))
}

// Bulk edit form field names. Per-row fields are namespaced by config key so a
// single form can carry edits for many rows. BulkKeysField enumerates which
// rows are present (one value per editable row).
const BulkKeysField = "keys"

func BulkValueField(key string) string { return "value:" + key }
func BulkOrigField(key string) string  { return "orig:" + key }
func BulkTypeField(key string) string  { return "type:" + key }
func BulkIDField(key string) string    { return "id:" + key }

// BulkConfigCell renders the value <td> for a row in a table that supports
// inline bulk editing. It contains both the read-only value display and a
// hidden editor (shown via the .editing class on an ancestor). All editor
// inputs are associated with the form identified by formID via the form
// attribute, so they can live outside that <form> element (avoiding nested
// forms from the per-row popovers). idForUpdate is the existing env-override
// config id, or empty when the row would create a new override.
func BulkConfigCell(formID, idForUpdate string, item ConfigItem) g.Node {
	hidden := func(name, value string) g.Node {
		return h.Input(h.Type("hidden"), h.Name(name), h.Value(value), g.Attr("form", formID))
	}
	display := ValueDisplay(item.Value, item.Type)
	switch {
	case !item.HasValue:
		display = h.Span(h.Class("value-empty"), g.Text("<not set>"))
	case item.IsSecret:
		display = h.Span(h.Class("text-muted"), g.Text("\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022"))
	case item.Value == "":
		display = h.Span(h.Class("value-empty"), g.Text("(empty string)"))
	}
	return h.Td(
		h.Div(h.Class("config-value-display"), display),
		h.Div(
			h.Class("config-value-edit"),
			bulkValueWidget(formID, item),
			hidden(BulkKeysField, item.Key),
			hidden(BulkTypeField(item.Key), item.Type),
			hidden(BulkOrigField(item.Key), item.Value),
			hidden(BulkIDField(item.Key), idForUpdate),
		),
	)
}

func bulkValueWidget(formID string, item ConfigItem) g.Node {
	name := BulkValueField(item.Key)
	switch strings.ToUpper(item.Type) {
	case "BOOL":
		opts := []g.Node{Option("true", item.Value), Option("false", item.Value)}
		if strings.TrimSpace(item.Value) == "" {
			opts = append([]g.Node{notSetOption()}, opts...)
		}
		return h.Select(h.Name(name), g.Attr("form", formID), h.Class("bulk-value-input"), g.Group(opts))
	case "INT":
		return h.Input(h.Type("text"), g.Attr("inputmode", "numeric"), h.Name(name),
			h.Value(item.Value), g.Attr("form", formID), h.Class("bulk-value-input"))
	default:
		if IsMultilineValue(item.Value) {
			rows := min(max(strings.Count(item.Value, "\n")+1, 2), 12)
			return h.Textarea(h.Name(name), g.Attr("form", formID), h.Class("bulk-value-input"),
				h.Rows(fmt.Sprintf("%d", rows)), g.Text(item.Value))
		}
		return h.Input(h.Type("text"), h.Name(name), h.Value(item.Value), g.Attr("form", formID),
			h.Class("bulk-value-input"))
	}
}

// ComputedValueCell renders a <td> with a computed value (template + rendered output).
func ComputedValueCell(item ConfigItem) g.Node {
	if item.IsSecret {
		return h.Td(h.Span(h.Class("text-muted"), g.Text("••••••••")))
	}
	value := item.Value
	if !item.HasValue {
		return h.Td(h.Span(h.Class("value-empty"), h.Title(item.Template), g.Text("<not set>")))
	}
	if value == "" {
		return h.Td(h.Span(h.Class("value-empty"), h.Title(item.Template), g.Text("(empty string)")))
	}
	if IsMultilineValue(value) {
		return h.Td(
			h.Code(h.Class("text-muted"), h.Title(item.Template), g.Text("computed")),
			ReadonlyValueTextarea(value),
		)
	}
	return h.Td(h.Code(h.Title(item.Template), g.Text(value)))
}

// TemplateCell renders a <td> with a computed value template (no rendered output).
func TemplateCell(item ConfigItem) g.Node {
	return h.Td(h.Class("template-cell"), h.Code(h.Class("template-code text-muted"), g.Text(item.Template)))
}

// ConfigActionsCell renders a <td> with the config-actions-col class.
// Callers pass action nodes (edit/delete popovers) as children.
func ConfigActionsCell(children ...g.Node) g.Node {
	return h.Td(h.Class("config-actions-col"), g.Group(children))
}

// ConfigKebab renders a kebab menu for a config row with a compare-across-environments action.
func ConfigKebab(featureName, configKey string, extraItems ...g.Node) g.Node {
	kebabID := "config-kebab-" + strings.ReplaceAll(configKey, ".", "-")
	compareURL := "/features/" + featureName + "/config-compare/" + url.QueryEscape(configKey)
	items := []g.Node{
		h.Button(h.Type("button"), h.Class("kebab-item"), g.Attr("data-lazy-modal", compareURL), g.Text("Compare across environments")),
	}
	items = append(items, extraItems...)
	return KebabWrap(kebabID, items)
}

// ValueDisplay renders a value, pretty-printing JSON when applicable.
func ValueDisplay(value, configType string) g.Node {
	display := value
	if configType == "STRING" || configType == "STRING_ARRAY" || configType == "" {
		if pretty, ok := TryPrettyJSON(value); ok {
			display = pretty
		}
	}
	if IsMultilineValue(display) {
		return ReadonlyValueTextarea(display)
	}
	return h.Span(g.Text(display))
}

// ConfigValueEditor renders the appropriate editor widget for a config value.
func ConfigValueEditor(item ConfigItem, currentValue string) g.Node {
	switch strings.ToUpper(item.Type) {
	case "BOOL":
		opts := []g.Node{Option("true", currentValue), Option("false", currentValue)}
		if strings.TrimSpace(currentValue) == "" {
			opts = append([]g.Node{notSetOption()}, opts...)
		}
		return g.Group([]g.Node{
			h.Label(g.Text("Value")),
			h.Select(append([]g.Node{h.Name("value")}, opts...)...),
		})
	case "INT":
		return g.Group([]g.Node{
			h.Label(g.Text("Value")),
			h.Input(h.Type("number"), h.Name("value"), h.Value(currentValue)),
		})
	default:
		return StringEditor(currentValue)
	}
}

// StringEditor renders a textarea with JSON/RAW mode toggle.
func StringEditor(currentValue string) g.Node {
	display := currentValue
	isJSON := false
	if pretty, ok := TryPrettyJSON(currentValue); ok {
		display = pretty
		isJSON = true
	}
	rows := min(max(strings.Count(display, "\n")+1, 4), 20)
	initialMode := "raw"
	if isJSON {
		initialMode = "json"
	}
	return g.Group([]g.Node{
		h.Div(
			h.Class("value-editor-toolbar"),
			h.Label(g.Text("Value")),
			h.Span(
				h.Class("mode-toggle"),
				h.Label(
					h.Input(
						h.Type("radio"), h.Name("mode"), h.Value("json"),
						g.If(initialMode == "json", g.Attr("checked", "checked")),
						g.Attr("data-mode-toggle", ""),
					),
					g.Text(" JSON"),
				),
				g.Text(" "),
				h.Label(
					h.Input(
						h.Type("radio"), h.Name("mode"), h.Value("raw"),
						g.If(initialMode == "raw", g.Attr("checked", "checked")),
						g.Attr("data-mode-toggle", ""),
					),
					g.Text(" RAW"),
				),
			),
		),
		h.Textarea(
			h.Name("value"),
			h.Rows(fmt.Sprintf("%d", rows)),
			g.Attr("data-mode-target", ""),
			g.Text(display),
		),
	})
}

// ConfigEditPopover renders an edit popover for a config item.
func ConfigEditPopover(popoverID, action, title, submitLabel string, item ConfigItem, extraFields ...g.Node) g.Node {
	formFields := append([]g.Node{}, extraFields...)
	return g.Group([]g.Node{
		h.Button(h.Type("button"), h.Class("edit-icon"), g.Attr("popovertarget", popoverID), g.Text("✎")),
		Popover(
			popoverID, "", title,
			h.Form(
				h.Method("POST"), h.Action(action),
				g.Group(formFields),
				h.Label(g.Text("Configuration Key")),
				h.Input(h.Type("text"), h.Value(item.Key), g.Attr("disabled", "")),
				ConfigValueEditor(item, item.Value),
				PopoverActions(
					h.Button(h.Type("submit"), g.Text(submitLabel)),
				),
			),
		),
	})
}

// ConfigDeletePopover renders a delete confirmation popover with an inline ✕
// trigger button (used in the per-row actions column on the global config page).
func ConfigDeletePopover(popoverID, action, title, confirmLabel, message, fallbackValue string) g.Node {
	return g.Group([]g.Node{
		h.Button(h.Type("button"), h.Class("edit-icon delete-icon"), g.Attr("popovertarget", popoverID), g.Text("✕")),
		ConfigDeleteConfirm(popoverID, action, title, confirmLabel, message, fallbackValue),
	})
}

// ConfigDeleteConfirm renders just the delete confirmation popover, without a
// trigger button. Callers provide their own trigger via popovertarget=popoverID
// (e.g. a kebab menu item).
func ConfigDeleteConfirm(popoverID, action, title, confirmLabel, message, fallbackValue string) g.Node {
	return Popover(
		popoverID, "", title,
		g.If(message != "", h.P(g.Text(message))),
		g.If(fallbackValue != "", fallbackValueNode(fallbackValue)),
		h.Form(
			h.Method("POST"), h.Action(action),
			PopoverActions(
				h.Button(h.Type("submit"), g.Text(confirmLabel)),
			),
		),
	)
}

func fallbackValueNode(v string) g.Node {
	if strings.Contains(v, "\n") {
		return g.Group([]g.Node{
			h.P(h.Strong(g.Text("Reverts to:"))),
			h.Pre(g.Text(v)),
		})
	}
	return h.P(
		h.Strong(g.Text("Reverts to: ")),
		h.Code(g.Text(v)),
	)
}

// notSetOption renders the selected placeholder option for an unset value,
// submitting an empty string so the bulk-save diff treats the row as unchanged.
func notSetOption() g.Node {
	return h.Option(h.Value(""), g.Attr("selected", "selected"), g.Text("(not set)"))
}

// Option renders a select option, marking it selected when it matches current.
func Option(value, current string) g.Node {
	attrs := []g.Node{h.Value(value)}
	if value == current {
		attrs = append(attrs, g.Attr("selected", "selected"))
	}
	return h.Option(append(attrs, g.Text(value))...)
}

// ReadonlyValueTextarea renders a read-only, non-editable block for multiline
// values. It is intentionally not a form control so it doesn't look editable.
func ReadonlyValueTextarea(value string) g.Node {
	return h.Pre(h.Class("value-readonly"), h.Code(g.Text(value)))
}

// IsMultilineValue returns true if the string contains newlines.
func IsMultilineValue(s string) bool {
	return strings.Contains(s, "\n")
}

// TryPrettyJSON attempts to pretty-print a JSON string.
func TryPrettyJSON(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s, false
	}
	switch trimmed[0] {
	case '{', '[':
	default:
		return s, false
	}
	var v any
	if err := json.Unmarshal([]byte(trimmed), &v); err != nil {
		return s, false
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return s, false
	}
	return string(b), true
}

// ParseConfigValue parses a form-submitted config value according to its type
// and editor mode ("json" or "raw").
func ParseConfigValue(value, configType, mode string) (any, error) {
	switch strings.ToUpper(configType) {
	case "INT":
		var intVal int
		if _, err := fmt.Sscan(value, &intVal); err != nil {
			return nil, err
		}
		return intVal, nil
	case "BOOL":
		switch strings.ToLower(value) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, fmt.Errorf("invalid bool")
		}
	case "STRING_ARRAY":
		if mode == "json" {
			var arr []string
			if err := json.Unmarshal([]byte(value), &arr); err != nil {
				return nil, fmt.Errorf("invalid JSON array: %w", err)
			}
			return arr, nil
		}
		var arr []string
		if err := json.Unmarshal([]byte(value), &arr); err == nil {
			return arr, nil
		}
		parts := strings.Split(value, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts, nil
	default:
		if mode == "json" {
			var v any
			if err := json.Unmarshal([]byte(value), &v); err != nil {
				return nil, fmt.Errorf("invalid JSON: %w", err)
			}
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			return string(b), nil
		}
		return value, nil
	}
}

// RawValueForDisplay converts a JSON raw message to a display string.
// Objects and arrays are pretty-printed; scalar strings are unquoted
// (and pretty-printed if they contain embedded JSON).
func RawValueForDisplay(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		var buf bytes.Buffer
		if json.Indent(&buf, []byte(s), "", "  ") == nil && len(s) > 0 && (s[0] == '{' || s[0] == '[') {
			return strings.TrimSpace(buf.String())
		}
		return s
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err == nil {
		return strings.TrimSpace(buf.String())
	}
	return strings.TrimSpace(string(raw))
}
