package environment

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// normalizedAttrs are attribute keys whose values vary per-row and should be replaced with "_".
var normalizedAttrs = map[string]bool{
	"id":                  true,
	"for":                 true,
	"action":              true,
	"value":               true,
	"popovertarget":       true,
	"popovertargetaction": true,
	"name":                true,
	"href":                true,
}

// skeleton returns a structural representation of an HTML node tree:
// text nodes are omitted (we only compare element structure and attributes),
// attribute values for per-row attrs are normalized to "_",
// and attributes are sorted alphabetically for deterministic output.
func skeleton(n *html.Node) string {
	var buf strings.Builder
	writeSkeleton(&buf, n)
	return buf.String()
}

func writeSkeleton(buf *strings.Builder, n *html.Node) {
	switch n.Type {
	case html.TextNode:
		// Skip text nodes entirely — we only care about structural/attribute differences.
		return
	case html.ElementNode:
		buf.WriteString("<")
		buf.WriteString(n.Data)
		if len(n.Attr) > 0 {
			attrs := make([]html.Attribute, len(n.Attr))
			copy(attrs, n.Attr)
			sort.Slice(attrs, func(i, j int) bool {
				return attrs[i].Key < attrs[j].Key
			})
			for _, a := range attrs {
				val := a.Val
				if normalizedAttrs[a.Key] {
					val = "_"
				}
				buf.WriteString(fmt.Sprintf(" %s=%q", a.Key, val))
			}
		}
		buf.WriteString(">")
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeSkeleton(buf, c)
		}
		buf.WriteString("</")
		buf.WriteString(n.Data)
		buf.WriteString(">")
	default:
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			writeSkeleton(buf, c)
		}
	}
}

// findElements finds all elements with the given tag name.
func findElements(n *html.Node, tag string) []*html.Node {
	var result []*html.Node
	if n.Type == html.ElementNode && n.Data == tag {
		result = append(result, n)
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		result = append(result, findElements(c, tag)...)
	}
	return result
}

func TestOverviewTab_RequiredUnsetConfigWarning(t *testing.T) {
	feat := &model.Feature{
		Name: "test-feature",
		FeatureYAML: model.FeatureYAML{
			Values: model.Values{
				"certificates": model.Value{
					DisplayName: "Certificates",
					Required:    true,
					Config:      &model.Config{Type: model.ConfigTypeString},
				},
				"max_replicas": model.Value{
					DisplayName: "Max Replicas",
					Required:    false,
					Config:      &model.Config{Type: model.ConfigTypeString},
				},
			},
		},
	}

	page := &FeaturePage{
		TenantSlug: "test-tenant",
		Environment: &Environment{
			Environment: &model.Environment{Name: "test-env"},
		},
		Feature: &FeatureDetail{
			Feature: feat,
			Enabled: true,
			ConfigItems: []FeatureConfigItem{
				{
					Key:            "certificates",
					DisplayName:    "Certificates",
					Value:          "",
					Source:         string(model.ConfigSourceHelm),
					Type:           "STRING",
					IsConfigurable: true,
				},
				{
					Key:            "max_replicas",
					DisplayName:    "Max Replicas",
					Value:          "250m",
					Source:         string(model.ConfigSourceHelm),
					Type:           "STRING",
					IsConfigurable: true,
				},
			},
		},
	}

	node := overviewTab(page)

	var buf bytes.Buffer
	err := node.Render(&buf)
	require.NoError(t, err, "overviewTab should render without error")

	doc, err := html.Parse(strings.NewReader(buf.String()))
	require.NoError(t, err, "rendered HTML should parse")

	trs := findElements(doc, "tr")
	require.GreaterOrEqual(t, len(trs), 3, "expected at least 3 <tr> elements (1 header + 2 data rows)")

	// Skip the first <tr> (header row in <thead>), data rows follow in fixture order.
	dataRows := trs[1:]
	require.GreaterOrEqual(t, len(dataRows), 2, "expected at least 2 data rows")

	// Determine which rows should warn based on the spec rule:
	// warn iff feat.FeatureYAML.Values[item.Key].Required && item.Source == "HELM" && item.Value == ""
	type rowInfo struct {
		skeleton   string
		shouldWarn bool
	}

	rows := make([]rowInfo, 2)
	for i, item := range page.Feature.ConfigItems {
		valDef := feat.FeatureYAML.Values[item.Key]
		warn := valDef.Required && item.Source == string(model.ConfigSourceHelm) && item.Value == ""
		rows[i] = rowInfo{
			skeleton:   skeleton(dataRows[i]),
			shouldWarn: warn,
		}
	}

	// Sanity: exactly one warn row and one non-warn row
	require.True(t, rows[0].shouldWarn, "certificates row should be flagged as warn")
	require.False(t, rows[1].shouldWarn, "max_replicas row should not be flagged as warn")

	// The warn row skeleton must differ from the non-warn row skeleton,
	// proving there is a visual warning indicator for the required-but-unset field.
	assert.NotEqual(t, rows[0].skeleton, rows[1].skeleton,
		"required-but-unset config row skeleton should differ from non-warn row skeleton, "+
			"indicating a visual warning indicator; but both rows have identical structure")
}

// TestOverviewTab_RequiredEmptyStructuredConfigWarning verifies that required
// HELM-sourced fields whose chart default is an "empty" JSON value (empty
// array, empty object, null) are also flagged as missing — not just literal
// empty strings.
func TestOverviewTab_RequiredEmptyStructuredConfigWarning(t *testing.T) {
	cases := []struct {
		name  string
		value string
		warn  bool
	}{
		{"empty string", "", true},
		{"empty array", "[]", true},
		{"empty object", "{}", true},
		{"json null", "null", true},
		{"non-empty array", `["a"]`, false},
		{"non-empty object", `{"k":"v"}`, false},
		{"zero number", "0", false},
		{"false bool", "false", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			feat := &model.Feature{
				Name: "f",
				FeatureYAML: model.FeatureYAML{
					Values: model.Values{
						"k": model.Value{Required: true, Config: &model.Config{Type: model.ConfigTypeString}},
					},
				},
			}
			page := &FeaturePage{
				TenantSlug:  "t",
				Environment: &Environment{Environment: &model.Environment{Name: "e"}},
				Feature: &FeatureDetail{
					Feature: feat,
					Enabled: true,
					ConfigItems: []FeatureConfigItem{{
						Key: "k", Value: tc.value,
						Source: string(model.ConfigSourceHelm),
						Type:   "STRING", IsConfigurable: true,
					}},
				},
			}
			var buf bytes.Buffer
			require.NoError(t, overviewTab(page).Render(&buf))
			doc, err := html.Parse(strings.NewReader(buf.String()))
			require.NoError(t, err)
			trs := findElements(doc, "tr")
			require.GreaterOrEqual(t, len(trs), 2)
			class := attrValue(trs[1], "class")
			hasWarn := strings.Contains(class, "config-warning")
			assert.Equal(t, tc.warn, hasWarn, "value %q: expected warn=%v, got class=%q", tc.value, tc.warn, class)
		})
	}
}

func attrValue(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
