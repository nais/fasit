package features

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featuresql"
)

func TestGlobalConfigurationShowsOnlyOverrides(t *testing.T) {
	for _, tt := range []struct {
		name       string
		configType featurepkg.ConfigType
		secret     bool
		override   json.RawMessage
		wantValue  string
	}{
		{name: "chart default is not a global override", configType: featurepkg.ConfigTypeString, wantValue: "&lt;not set&gt;"},
		{name: "unset secret is not masked as a set value", configType: featurepkg.ConfigTypeString, secret: true, wantValue: "&lt;not set&gt;"},
		{name: "unset boolean is not preselected", configType: featurepkg.ConfigTypeBool, wantValue: "&lt;not set&gt;"},
		{name: "explicit string", configType: featurepkg.ConfigTypeString, override: json.RawMessage(`"global-value"`), wantValue: "global-value"},
		{name: "explicit false", configType: featurepkg.ConfigTypeBool, override: json.RawMessage(`false`), wantValue: ">false<"},
		{name: "explicit empty string", configType: featurepkg.ConfigTypeString, override: json.RawMessage(`""`), wantValue: "(empty string)"},
		{name: "set secret stays masked", configType: featurepkg.ConfigTypeString, secret: true, override: json.RawMessage(`"secret-value"`), wantValue: "••••••••"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			feat := &featurepkg.Feature{
				Name: "pgrator",
				FeatureYAML: featurepkg.FeatureYAML{
					Values: featurepkg.Values{
						"key": {Config: &featurepkg.Config{Type: tt.configType, Secret: tt.secret}},
					},
				},
				ValuesYAML: map[string]json.RawMessage{"key": json.RawMessage(`"chart-default"`)},
			}
			if tt.configType == featurepkg.ConfigTypeBool {
				feat.ValuesYAML["key"] = json.RawMessage(`false`)
			}
			q := &globalConfigQuerier{}
			if tt.override != nil {
				q.configs = []featuresql.ConfigurationsGlobal{{
					ID: uuid.New(), Feature: feat.Name, Key: "key", Value: tt.override,
				}}
			}
			ctx := context.WithValue(context.Background(), featurepkg.QuerierKey, q)
			items, err := loadGlobalConfigItems(ctx, feat)
			if err != nil {
				t.Fatalf("load global config: %v", err)
			}
			var buf bytes.Buffer
			if err := globalConfigurableTable(feat.Name, items).Render(&buf); err != nil {
				t.Fatalf("render global config: %v", err)
			}
			html := buf.String()
			if !strings.Contains(html, tt.wantValue) {
				t.Errorf("global configuration is missing %q", tt.wantValue)
			}
			for _, unwanted := range []string{"chart-default", "values.yaml"} {
				if strings.Contains(html, unwanted) {
					t.Errorf("global configuration should not show %q", unwanted)
				}
			}
			if tt.override == nil {
				for _, unwanted := range []string{"••••••••", `value="false" selected`, ">global config<"} {
					if strings.Contains(html, unwanted) {
						t.Errorf("unset global configuration should not contain %q", unwanted)
					}
				}
			} else {
				if strings.Contains(html, "&lt;not set&gt;") {
					t.Error("explicit global override rendered as unset")
				}
				if !strings.Contains(html, ">global config<") {
					t.Error("missing global config source")
				}
			}
		})
	}
}

type globalConfigQuerier struct {
	featuresql.Querier
	configs []featuresql.ConfigurationsGlobal
}

func (q *globalConfigQuerier) ListGlobalConfigByFeature(context.Context, string) ([]featuresql.ConfigurationsGlobal, error) {
	return q.configs, nil
}
