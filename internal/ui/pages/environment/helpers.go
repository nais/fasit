package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/deployment"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/view"
	yaml "gopkg.in/yaml.v3"
)

type FeatureConfigItem struct {
	ID          string
	Key         string
	DisplayName string
	Description string
	Value       string
	Source      string
	Type        string
	IsSecret    bool
	IsComputed  bool
	Template    string
}

type FeaturePage struct {
	Breadcrumbs      []breadcrumb.Crumb
	Tenant           *model.Tenant
	TenantSlug       string
	Environment      *Environment
	Feature          *FeatureDetail
	AllFeatures      []view.FeatureNav
	EnabledFeatures  []view.FeatureNav
	HelmValues       string
	Rollouts         []RolloutItem
	FeatureLog       *FeatureLog
	ActiveTab        string
	PlaygroundCode   string
	PlaygroundResult *PlaygroundResult
}

type FeatureDetail struct {
	*model.Feature
	Enabled     bool
	ConfigItems []FeatureConfigItem
}

type RolloutItem struct {
	FeatureName  string
	Version      string
	Status       string
	Created      string
	Completed    string
	Target       string
	DeploymentID string
}

type FeatureLog struct {
	CurrentVersion string
	CurrentStatus  string
	LastModified   string
	CurrentLog     []LogLine
	HelmDiff       *model.HelmValueDiff
}

type LogLine struct {
	Timestamp string
	Message   string
}

type PlaygroundResult struct {
	Result string
	Errors []string
}

func toTenantNavs(tenants []*model.Tenant) []view.TenantNav {
	ret := make([]view.TenantNav, 0, len(tenants))
	for _, tenant := range tenants {
		ret = append(ret, view.TenantNav{Name: tenant.Name})
	}
	return ret
}

func toEnvironmentNavs(environments []*model.Environment) []view.EnvironmentNav {
	ret := make([]view.EnvironmentNav, 0, len(environments))
	for _, env := range environments {
		ret = append(ret, view.EnvironmentNav{Name: env.Name})
	}
	return ret
}

func featureNavs(ctx context.Context, env *model.Environment) ([]view.FeatureNav, []view.FeatureNav, error) {
	features, err := featurepkg.FeaturesForKind(ctx, env.Kind, env.CI)
	if err != nil {
		return nil, nil, err
	}

	allFeatures := make([]view.FeatureNav, 0, len(features))
	enabledFeatures := make([]view.FeatureNav, 0, len(features))
	for _, feat := range features {
		state, err := featurepkg.FeatureStateGet(ctx, env.ID, feat.Name)
		if err != nil {
			return nil, nil, err
		}
		nav := view.FeatureNav{Name: feat.Name, Enabled: state.Enabled}
		allFeatures = append(allFeatures, nav)
		enabledFeatures = append(enabledFeatures, nav)
	}
	return allFeatures, enabledFeatures, nil
}

func getEnvironmentMetadata(ctx context.Context, repo database.Repo, env *model.Environment) []MetadataItem {
	metadata := []MetadataItem{}
	addMetadata(&metadata, "ID", env.ID.String())
	addMetadata(&metadata, "Name", env.Name)
	if env.Description != nil {
		addMetadata(&metadata, "Description", *env.Description)
	}
	addMetadata(&metadata, "Kind", env.Kind.String())
	addMetadata(&metadata, "Created", formatTime(env.Created))
	addMetadata(&metadata, "Last Modified", formatTime(env.LastModified))
	addMetadataBool(&metadata, "Reconcile", env.Reconcile)

	values, err := repo.EnvironmentValuesForEnvironment(ctx, env.ID, true)
	if err == nil {
		for _, val := range values {
			addMetadata(&metadata, val.Key, rawValueToString(val.Value))
		}
	}

	return metadata
}

func addMetadata(items *[]MetadataItem, key, value string) {
	if value != "" {
		*items = append(*items, MetadataItem{Key: key, Value: value})
	}
}

func addMetadataBool(items *[]MetadataItem, key string, value bool) {
	*items = append(*items, MetadataItem{Key: key, Value: fmt.Sprintf("%t", value)})
}

func rawValueToString(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return string(value)
	}
	switch typed := v.(type) {
	case string:
		return typed
	default:
		b, err := json.Marshal(typed)
		if err != nil {
			return string(value)
		}
		return string(b)
	}
}

func rawValueForInput(value json.RawMessage) string {
	if len(value) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(value, &v); err != nil {
		return string(value)
	}
	switch typed := v.(type) {
	case string:
		return typed
	default:
		b, err := json.MarshalIndent(typed, "", "  ")
		if err != nil {
			return string(value)
		}
		return string(b)
	}
}

func countEnabled(features []view.FeatureNav) int {
	count := 0
	for _, feature := range features {
		if feature.Enabled {
			count++
		}
	}
	return count
}

func loadFeaturePageData(ctx context.Context, repo database.Repo, tenantSlug, envName, featureName, activeTab string) (*FeaturePage, error) {
	tenant, err := envpkg.GetTenantGetByName(ctx, tenantSlug)
	if err != nil {
		return nil, err
	}

	env, err := repo.EnvironmentGetByName(ctx, tenant.ID, envName)
	if err != nil {
		return nil, err
	}

	allFeatures, enabledFeatures, err := featureNavs(ctx, env)
	if err != nil {
		return nil, err
	}

	feat, err := featurepkg.FeatureByNameForEnv(ctx, featureName, env.ID)
	if err != nil {
		return nil, err
	}

	state, err := featurepkg.FeatureStateGet(ctx, env.ID, featureName)
	if err != nil {
		return nil, err
	}

	allTenants, _ := envpkg.GetTenants(ctx)
	tenantEnvs, _ := repo.EnvironmentsGet(ctx, tenant.ID)

	page := &FeaturePage{
		Breadcrumbs: []breadcrumb.Crumb{
			breadcrumb.TenantWithSwitcher(tenant.Name, toTenantNavs(allTenants)),
			breadcrumb.EnvironmentWithSwitcher(tenant.Name, env.Name, toEnvironmentNavs(tenantEnvs)),
			breadcrumb.EnvironmentFeature(tenant.Name, env.Name, featureName),
		},
		Tenant:          tenant,
		TenantSlug:      tenantSlug,
		Environment:     &Environment{Environment: env, Metadata: getEnvironmentMetadata(ctx, repo, env)},
		Feature:         &FeatureDetail{Feature: feat, Enabled: state.Enabled},
		AllFeatures:     allFeatures,
		EnabledFeatures: enabledFeatures,
		ActiveTab:       activeTab,
	}

	page.Feature.ConfigItems, err = loadFeatureConfigItems(ctx, feat, env.ID)
	if err != nil {
		return nil, err
	}

	if activeTab == "helm" || activeTab == "playground" {
		page.HelmValues, _ = loadHelmValues(ctx, feat, env.ID)
	}
	if activeTab == "rollouts" {
		page.Rollouts = loadEnvironmentRollouts(ctx, repo, featureName)
	}
	if activeTab == "logs" {
		page.FeatureLog = loadFeatureLog(ctx, repo, env.ID, featureName)
	}

	return page, nil
}

func loadFeatureConfigItems(ctx context.Context, feat *model.Feature, envID uuid.UUID) ([]FeatureConfigItem, error) {
	configs, err := featurepkg.EnvConfig(ctx, feat, envID)
	if err != nil {
		return nil, err
	}

	for key, val := range feat.Values {
		if val.Config == nil {
			continue
		}
		found := false
		for _, cfg := range configs {
			if cfg.Key == key {
				cfg.Value = &val
				found = true
				break
			}
		}
		if !found {
			value := val
			configs = append(configs, &model.Configuration{
				ID:      uuid.NewSHA1(uuid.Nil, []byte(feat.Name+"|"+key)),
				Key:     key,
				Value:   &value,
				Content: feat.ValuesYAML[key],
				Source:  model.ConfigSourceHelm,
			})
		}
	}

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Key < configs[j].Key
	})

	items := make([]FeatureConfigItem, 0, len(configs))
	for _, cfg := range configs {
		item := FeatureConfigItem{
			ID:     cfg.ID.String(),
			Key:    cfg.Key,
			Source: string(cfg.Source),
			Value:  rawValueForInput(cfg.Content),
		}
		if cfg.Value != nil {
			item.DisplayName = cfg.Value.DisplayName
			item.Description = cfg.Value.Description
			if cfg.Value.Config != nil {
				item.Type = strings.ToUpper(cfg.Value.Config.Type.String())
				item.IsSecret = cfg.Value.Config.Secret
			}
			if cfg.Value.Computed != nil {
				item.IsComputed = true
				item.Template = cfg.Value.Computed.Template
			}
		}
		items = append(items, item)
	}

	return items, nil
}

func loadHelmValues(ctx context.Context, feat *model.Feature, envID uuid.UUID) (string, error) {
	vals, err := featurepkg.HelmValues(ctx, feat, envID)
	if err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(vals, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func loadEnvironmentRollouts(ctx context.Context, repo database.Repo, featureName string) []RolloutItem {
	items := []RolloutItem{}
	rollouts, err := repo.RolloutsForFeature(ctx, featureName)
	if err == nil {
		for _, rollout := range rollouts {
			items = append(items, RolloutItem{
				FeatureName: rollout.FeatureName,
				Version:     rollout.Version,
				Status:      strings.ToUpper(rollout.Status.String()),
				Created:     formatTime(rollout.Created),
				Completed:   formatTimePtr(rollout.Completed),
			})
		}
	}

	deployments, err := deployment.ListDeploymentsByFeature(ctx, featureName)
	if err == nil {
		for _, dep := range deployments {
			items = append(items, RolloutItem{
				FeatureName:  dep.Feature.Name,
				Version:      dep.Feature.Version,
				Status:       "DEPLOYMENT",
				Created:      formatTime(dep.Created),
				Target:       deploymentTarget(dep),
				DeploymentID: dep.ID.String(),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Created > items[j].Created
	})
	return items
}

func deploymentTarget(dep *deployment.Deployment) string {
	if dep.CI {
		return "CI"
	}
	labels := dep.Target()
	if len(labels) == 0 {
		return "All environments"
	}
	parts := make([]string, 0, len(labels))
	for _, label := range labels {
		parts = append(parts, label.Key+"="+label.Value)
	}
	return strings.Join(parts, ", ")
}

func loadFeatureLog(ctx context.Context, repo database.Repo, envID uuid.UUID, featureName string) *FeatureLog {
	di, err := repo.DeployInstructionsLatestForFeature(ctx, envID, featureName)
	if err != nil {
		return nil
	}
	lines, err := featurepkg.LogsGet(ctx, di.ID)
	if err != nil {
		return nil
	}
	ret := &FeatureLog{
		CurrentVersion: di.FeatureVersion,
		CurrentStatus:  strings.ToUpper(di.Status.String()),
		LastModified:   formatTime(di.LastModified),
	}
	for _, line := range lines {
		ret.CurrentLog = append(ret.CurrentLog, LogLine{Timestamp: formatTime(line.Timestamp), Message: line.Message})
	}
	ret.HelmDiff, _ = repo.HelmValueDiffGet(ctx, di)
	return ret
}

func parseConfigValue(value, configType string) (any, error) {
	switch configType {
	case "INT":
		var intVal int
		if _, err := fmt.Sscan(value, &intVal); err != nil {
			return nil, err
		}
		return intVal, nil
	case "BOOL":
		lower := strings.ToLower(value)
		if lower == "true" {
			return true, nil
		}
		if lower == "false" {
			return false, nil
		}
		return nil, fmt.Errorf("invalid bool")
	case "STRING_ARRAY":
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
		return value, nil
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(oslo).Format("2006-01-02 15:04:05")
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return formatTime(*t)
}

var oslo = mustLoadLocation("Europe/Oslo")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func stripNoValue(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			stripNoValue(val)
			if len(val) == 0 {
				delete(m, k)
			}
		case string:
			if val == "<no value>" {
				m[k] = nil
			}
		}
	}
}

func yamlUnmarshalFeature(code string, out *model.FeatureYAML) error {
	return yaml.Unmarshal([]byte(code), out)
}

func runPlayground(ctx context.Context, repo database.Repo, tenantSlug, envSlug, featureName, code string, includeUnset bool) (*PlaygroundResult, error) {
	env, err := repo.EnvironmentByNames(ctx, tenantSlug, envSlug)
	if err != nil {
		return &PlaygroundResult{Errors: []string{err.Error()}}, nil
	}

	featureYAML := model.FeatureYAML{}
	if err := yamlUnmarshalFeature(code, &featureYAML); err != nil {
		return &PlaygroundResult{Errors: []string{err.Error()}}, nil
	}

	for key, value := range featureYAML.Values {
		value.Required = false
		featureYAML.Values[key] = value
	}

	feat := &model.Feature{FeatureYAML: featureYAML, Name: featureName}
	vals, err := featurepkg.HelmValues(ctx, feat, env.ID)
	if err != nil {
		return &PlaygroundResult{Errors: []string{err.Error()}}, nil
	}

	stripNoValue(vals)
	if includeUnset {
		for key, value := range feat.Values {
			if value.Config == nil || value.Computed != nil {
				continue
			}
			parts, err := featureutil.SmartDotSplit(key)
			if err != nil {
				return &PlaygroundResult{Errors: []string{err.Error()}}, nil
			}
			outer := vals
			for i, part := range parts {
				if i == len(parts)-1 {
					if _, ok := outer[part]; !ok {
						outer[part] = nil
					}
					break
				}
				if _, ok := outer[part]; !ok {
					outer[part] = map[string]any{}
				}
				next, ok := outer[part].(map[string]any)
				if !ok {
					break
				}
				outer = next
			}
		}
	}

	b, err := json.MarshalIndent(vals, "", "  ")
	if err != nil {
		return &PlaygroundResult{Errors: []string{err.Error()}}, nil
	}
	return &PlaygroundResult{Result: string(b)}, nil
}
