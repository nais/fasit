package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
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
	Template       string
	MappedCount    int
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
	Deployments      []EnvDeploymentItem
	FeatureLog       *FeatureLog
	ActiveTab        string
	PlaygroundCode   string
	PlaygroundResult *PlaygroundResult
	AuditEntries     []*audit.Entry
}

type FeatureDetail struct {
	*model.Feature
	Enabled     bool
	ConfigItems []FeatureConfigItem
}

type RolloutItem struct {
	FeatureName string
	Version     string
	Status      string
	Created     string
	Completed   string
	Target      string
}

type EnvDeploymentItem struct {
	ID           string
	Version      string
	Status       string
	TargetLabels map[string]string
	Created      string
}

type FeatureLog struct {
	CurrentVersion string
	CurrentStatus  string
	LastModified   string
	LastDeployed   string
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

func featureNavs(ctx context.Context, repo database.Repo, env *model.Environment) ([]view.FeatureNav, []view.FeatureNav, error) {
	deploymentFeatures, err := deployment.ListEnvironmentFeatures(ctx, env.ID)
	if err != nil {
		return nil, nil, err
	}

	disabled, err := featurepkg.FeaturesDisabledIn(ctx, env.ID)
	if err != nil {
		return nil, nil, err
	}

	seen := make(map[string]bool, len(deploymentFeatures))
	var names []string
	for _, f := range deploymentFeatures {
		if seen[f.FeatureName] {
			continue
		}
		seen[f.FeatureName] = true
		names = append(names, f.FeatureName)
	}

	// include features controlled by rollouts
	featureStates, err := featurepkg.FeatureStatesGet(ctx, env.ID)
	if err != nil {
		return nil, nil, err
	}
	for _, state := range featureStates {
		if seen[state.FeatureName] {
			continue
		}
		seen[state.FeatureName] = true
		names = append(names, state.FeatureName)
	}

	allFeatures := make([]view.FeatureNav, 0, len(names))
	enabledFeatures := make([]view.FeatureNav, 0, len(names))
	for _, name := range names {
		nav := view.FeatureNav{Name: name}
		if _, ok := disabled[name]; !ok {
			nav.Enabled = true
		}
		allFeatures = append(allFeatures, nav)
		enabledFeatures = append(enabledFeatures, nav)
	}
	sort.Slice(allFeatures, func(i, j int) bool {
		return allFeatures[i].Name < allFeatures[j].Name
	})
	sort.Slice(enabledFeatures, func(i, j int) bool {
		return enabledFeatures[i].Name < enabledFeatures[j].Name
	})
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
	addMetadata(&metadata, "Created", view.FormatTime(env.Created))
	addMetadata(&metadata, "Last Modified", view.FormatTime(env.LastModified))
	addMetadataBool(&metadata, "Reconcile", env.Reconcile)

	values, err := repo.EnvironmentValuesForEnvironment(ctx, env.ID, true)
	if err == nil {
		for _, val := range values {
			if val.Secret {
				metadata = append(metadata, MetadataItem{Key: val.Key, Value: "", IsSecret: true})
			} else {
				addMetadata(&metadata, val.Key, rawValueToString(val.Value))
			}
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

func gcpProjectIDFromMetadata(metadata []MetadataItem) string {
	for _, m := range metadata {
		if m.Key == "project_id" && !m.IsSecret {
			return m.Value
		}
	}
	return ""
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

	allFeatures, enabledFeatures, err := featureNavs(ctx, repo, env)
	if err != nil {
		return nil, err
	}

	feat, err := featurepkg.FeatureByNameForEnv(ctx, featureName, env.ID)
	if err != nil {
		return nil, err
	}

	_, disabled, err := featurepkg.FeatureDisabledAt(ctx, env.ID, featureName)
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
		Feature:         &FeatureDetail{Feature: feat, Enabled: !disabled},
		AllFeatures:     allFeatures,
		EnabledFeatures: enabledFeatures,
		ActiveTab:       activeTab,
	}

	page.Feature.ConfigItems, err = loadFeatureConfigItems(ctx, feat, env.ID)
	if err != nil {
		return nil, err
	}

	page.FeatureLog = loadFeatureLog(ctx, repo, env.ID, feat)

	if activeTab == "helm" || activeTab == "playground" {
		page.HelmValues, _ = loadHelmValues(ctx, feat, env.ID)
	}
	if activeTab == "rollouts" {
		page.Rollouts = loadEnvironmentRollouts(ctx, repo, featureName)
	}
	if activeTab == "deployments" {
		page.Deployments = loadEnvironmentDeployments(ctx, repo, featureName, env.ID)
	}
	if activeTab == "audit" {
		entries, err := audit.EntriesForEnvironment(ctx, env.ID, featureName)
		if err != nil {
			return nil, fmt.Errorf("load audit entries: %w", err)
		}
		page.AuditEntries = entries
	}

	return page, nil
}

func loadFeatureConfigItems(ctx context.Context, feat *model.Feature, envID uuid.UUID) ([]FeatureConfigItem, error) {
	configs, err := featurepkg.EnvConfig(ctx, feat, envID)
	if err != nil {
		return nil, err
	}

	for key, val := range feat.Values {
		if val.Config == nil && val.Computed == nil {
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
	hasComputed := false
	for _, cfg := range configs {
		item := FeatureConfigItem{
			ID:          cfg.ID.String(),
			Key:         cfg.Key,
			Source:      string(cfg.Source),
			Value:       rawValueForInput(cfg.Content),
			MappedCount: countTemplateRefs(feat.Values, cfg.Key),
		}
		if cfg.Value != nil {
			item.DisplayName = cfg.Value.DisplayName
			item.Description = cfg.Value.Description
			if cfg.Value.Config != nil {
				item.Type = strings.ToUpper(cfg.Value.Config.Type.String())
				item.IsSecret = cfg.Value.Config.Secret
				item.IsConfigurable = true
			}
			if cfg.Value.Computed != nil {
				item.IsComputed = true
				item.Template = cfg.Value.Computed.Template
				hasComputed = true
			}
		}
		items = append(items, item)
	}

	if hasComputed {
		rendered, err := featurepkg.HelmValues(ctx, feat, envID)
		if err == nil {
			for i, item := range items {
				if !item.IsComputed {
					continue
				}
				if item.Source == string(model.ConfigSourceEnv) {
					continue
				}
				if v, ok := lookupHelmValue(rendered, item.Key); ok {
					items[i].Value = v
				}
			}
		}
	}

	return items, nil
}

func countTemplateRefs(values model.Values, key string) int {
	needle := ".Configs." + key
	count := 0
	for k, v := range values {
		if k == key || v.Computed == nil || v.Computed.Template == "" {
			continue
		}
		if strings.Contains(v.Computed.Template, needle) {
			count++
		}
	}
	return count
}

func lookupHelmValue(m map[string]any, key string) (string, bool) {
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
		b, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(b), true
	}
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
	rollouts, err := repo.RolloutsForFeature(ctx, featureName)
	if err != nil {
		return nil
	}
	items := make([]RolloutItem, 0, len(rollouts))
	for _, rollout := range rollouts {
		items = append(items, RolloutItem{
			FeatureName: rollout.FeatureName,
			Version:     rollout.Version,
			Status:      strings.ToUpper(rollout.Status.String()),
			Created:     view.FormatTime(rollout.Created),
			Completed:   view.FormatTimePtr(rollout.Completed),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Created > items[j].Created
	})
	return items
}

func loadEnvironmentDeployments(ctx context.Context, repo database.Repo, featureName string, envID uuid.UUID) []EnvDeploymentItem {
	envLabels, err := repo.EnvironmentGetLabels(ctx, envID)
	if err != nil {
		return nil
	}
	envLabelMap := make(map[string]string, len(envLabels))
	for _, l := range envLabels {
		envLabelMap[l.Key] = l.Value
	}

	deployments, err := deployment.ListDeploymentsByFeature(ctx, featureName)
	if err != nil {
		return nil
	}

	type candidate struct {
		id      string
		version string
		target  map[string]string
		created string
		status  string
	}

	var candidates []candidate
	maxLabels := 0

	for _, dep := range deployments {
		target := dep.TargetLabels
		if !labelsMatch(envLabelMap, target) {
			continue
		}

		status := "UNKNOWN"
		statuses, err := deployment.ListDeploymentStatuses(ctx, dep.ID)
		if err == nil {
			for _, s := range statuses {
				if s.EnvironmentID == envID {
					status = string(s.State)
					break
				}
			}
		}

		candidates = append(candidates, candidate{
			id:      dep.ID.String(),
			version: dep.Feature.Version,
			target:  target,
			created: view.FormatTime(dep.Created),
			status:  status,
		})
		if len(target) > maxLabels {
			maxLabels = len(target)
		}
	}

	items := make([]EnvDeploymentItem, 0, len(candidates))
	for _, c := range candidates {
		if len(c.target) == maxLabels {
			items = append(items, EnvDeploymentItem{
				ID:           c.id,
				Version:      c.version,
				Status:       c.status,
				TargetLabels: c.target,
				Created:      c.created,
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].Created > items[j].Created
	})
	return items
}

func labelsMatch(envLabels, target map[string]string) bool {
	for k, v := range target {
		if envLabels[k] != v {
			return false
		}
	}
	return true
}

func loadFeatureLog(ctx context.Context, repo database.Repo, envID uuid.UUID, feat *model.Feature) *FeatureLog {
	di, err := repo.DeployInstructionsLatestForFeature(ctx, envID, feat.Name)
	if err != nil {
		return nil
	}
	lines, err := featurepkg.LogsGet(ctx, di.ID)
	if err != nil {
		return nil
	}
	lastDeployed := "never"
	if dep, err := repo.DeployInstructionsLatestDeployedForFeature(ctx, envID, feat.Name); err == nil && dep != nil {
		lastDeployed = view.FormatTime(dep.LastModified)
	}
	ret := &FeatureLog{
		CurrentVersion: di.FeatureVersion,
		CurrentStatus:  strings.ToUpper(di.Status.String()),
		LastModified:   view.FormatTime(di.LastModified),
		LastDeployed:   lastDeployed,
	}
	for _, line := range lines {
		ret.CurrentLog = append(ret.CurrentLog, LogLine{Timestamp: view.FormatTime(line.Timestamp), Message: line.Message})
	}
	ret.HelmDiff, _ = repo.HelmValueDiffGet(ctx, di, feat.SecretKeys())
	return ret
}

func parseConfigValue(value, configType, mode string) (any, error) {
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
