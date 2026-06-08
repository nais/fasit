package environment

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"

	environment2 "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/featureenvs"
	"golang.org/x/net/html"
)

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
				_, _ = fmt.Fprintf(buf, " %s=%q", a.Key, val)
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

func TestFeatureSidebarUsesFeatureEnvironmentList(t *testing.T) {
	page := &FeaturePage{
		Feature:     &FeatureDetail{Feature: &feature.Feature{Name: "azureator-nav"}},
		TenantSlug:  "nav",
		Environment: &Environment{Environment: &environment2.Environment{Name: "dev-fss"}},
		FeatureEnvs: []featureenvs.Environment{
			{TenantName: "ci-nais", TenantSlug: "ci-nais", EnvironmentName: "ci-fss", Status: "DEPLOYED"},
			{TenantName: "nav", TenantSlug: "nav", EnvironmentName: "dev", Status: "DEPLOYED"},
			{TenantName: "nav", TenantSlug: "nav", EnvironmentName: "dev-fss", Status: "DEPLOYED"},
			{TenantName: "nav", TenantSlug: "nav", EnvironmentName: "prod", Status: "DEPLOYED"},
			{TenantName: "nav", TenantSlug: "nav", EnvironmentName: "prod-fss", Status: "DEPLOYED"},
		},
	}

	var buf bytes.Buffer
	if err := featurePageSidebar(page).Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}

	output := buf.String()
	for _, label := range []string{"ci-nais / ci-fss", "nav / dev", "nav / dev-fss", "nav / prod", "nav / prod-fss"} {
		if !strings.Contains(output, label) {
			t.Errorf("missing expected label %q", label)
		}
	}
	for _, label := range []string{"atil / dev", "dev-nais / dev", "ssb / prod", "test-nais / sandbox"} {
		if strings.Contains(output, label) {
			t.Errorf("unexpected label %q present", label)
		}
	}
	if c := strings.Count(output, `class="feature-env-link active"`); c != 1 {
		t.Errorf("active link count = %d, want 1", c)
	}
}

func TestOverviewTab_RequiredUnsetConfigWarning(t *testing.T) {
	feat := &feature.Feature{
		Name: "test-feature",
		FeatureYAML: feature.FeatureYAML{
			Values: feature.Values{
				"certificates": feature.Value{
					DisplayName: "Certificates",
					Required:    true,
					Config:      &feature.Config{Type: feature.ConfigTypeString},
				},
				"max_replicas": feature.Value{
					DisplayName: "Max Replicas",
					Required:    false,
					Config:      &feature.Config{Type: feature.ConfigTypeString},
				},
			},
		},
	}

	page := &FeaturePage{
		TenantSlug: "test-tenant",
		Environment: &Environment{
			Environment: &environment2.Environment{Name: "test-env"},
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
	if err := node.Render(&buf); err != nil {
		t.Fatalf("overviewTab render: %v", err)
	}

	doc, err := html.Parse(strings.NewReader(buf.String()))
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}

	trs := findElements(doc, "tr")
	if len(trs) < 3 {
		t.Fatalf("got %d <tr> elements, want at least 3 (1 header + 2 data rows)", len(trs))
	}

	dataRows := trs[1:]
	if len(dataRows) < 2 {
		t.Fatalf("got %d data rows, want at least 2", len(dataRows))
	}

	type rowInfo struct {
		skeleton   string
		shouldWarn bool
	}

	rows := make([]rowInfo, 2)
	for i, item := range page.Feature.ConfigItems {
		valDef := feat.Values[item.Key]
		warn := valDef.Required && item.Source == string(model.ConfigSourceHelm) && item.Value == ""
		rows[i] = rowInfo{
			skeleton:   skeleton(dataRows[i]),
			shouldWarn: warn,
		}
	}

	if !rows[0].shouldWarn {
		t.Fatal("certificates row should be flagged as warn")
	}
	if rows[1].shouldWarn {
		t.Fatal("max_replicas row should not be flagged as warn")
	}

	if rows[0].skeleton == rows[1].skeleton {
		t.Error("required-but-unset config row skeleton should differ from non-warn row skeleton, " +
			"indicating a visual warning indicator; but both rows have identical structure")
	}
}

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
			feat := &feature.Feature{
				Name: "f",
				FeatureYAML: feature.FeatureYAML{
					Values: feature.Values{
						"k": feature.Value{Required: true, Config: &feature.Config{Type: feature.ConfigTypeString}},
					},
				},
			}
			page := &FeaturePage{
				TenantSlug:  "t",
				Environment: &Environment{Environment: &environment2.Environment{Name: "e"}},
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
			if err := overviewTab(page).Render(&buf); err != nil {
				t.Fatalf("render: %v", err)
			}
			doc, err := html.Parse(strings.NewReader(buf.String()))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			trs := findElements(doc, "tr")
			if len(trs) < 2 {
				t.Fatalf("got %d <tr>, want >= 2", len(trs))
			}
			class := attrValue(trs[1], "class")
			hasWarn := strings.Contains(class, "config-warning")
			if hasWarn != tc.warn {
				t.Errorf("value %q: expected warn=%v, got class=%q", tc.value, tc.warn, class)
			}
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
