package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	envpkg "github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/feature/featureutil"
	"github.com/nais/fasit/internal/featureassignment"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/ui/breadcrumb"
	"github.com/nais/fasit/internal/ui/components"
	"github.com/nais/fasit/internal/ui/featureenvs"
	"github.com/nais/fasit/internal/ui/uidata"
	"github.com/nais/fasit/internal/ui/view"
)

type FeatureConfigItem = components.ConfigItem

type FeaturePage struct {
	Breadcrumbs             []breadcrumb.Crumb
	Tenant                  *envpkg.Tenant
	TenantSlug              string
	Environment             *Environment
	Feature                 *FeatureDetail
	FeatureEnvs             []featureenvs.Environment
	Assignments             []EnvAssignmentItem
	FeatureLog              *FeatureLog
	Status                  string
	StatusMessage           string
	ActiveTab               string
	AuditEntries            []*audit.Entry
	WinningAssignment       *featureassignment.FeatureAssignment
	RecentDeployHistory     []*featurepkg.DeployInstruction
	DeployLogsByInstruction map[string][]LogLine
	ExpandedLogID           string
	ShowAllDeploys          bool
}

type FeatureDetail struct {
	*featurepkg.Feature
	Enabled       bool
	DisableReason string
	ConfigItems   []FeatureConfigItem
}

type EnvAssignmentItem struct {
	ID           string
	Version      string
	Status       string
	TargetLabels map[string]string
	Created      string
}

type FeatureLog struct {
	CurrentVersion       string
	CurrentStatus        string
	LastModified         string
	LastModifiedRelative string
	LastDeployed         string
	CurrentLog           []LogLine
	HelmDiff             *model.HelmValueDiff
}

type LogLine struct {
	Timestamp string
	Message   string
}

func tenantCrumb(name string, allTenants []view.TenantNav) breadcrumb.Crumb {
	c := breadcrumb.TenantWithSwitcher(name, allTenants)
	c.Icon = components.TenantAvatar(name, components.HasTenantLogo(name), "18px")
	return c
}

func toTenantNavs(tenants []*uidata.Tenant) []view.TenantNav {
	ret := make([]view.TenantNav, 0, len(tenants))
	for _, tenant := range tenants {
		ret = append(ret, view.TenantNav{Name: tenant.Name})
	}
	return ret
}

func toEnvironmentNavs(environments []*envpkg.Environment) []view.EnvironmentNav {
	ret := make([]view.EnvironmentNav, 0, len(environments))
	for _, env := range environments {
		ret = append(ret, view.EnvironmentNav{Name: env.Name})
	}
	return ret
}

func getEnvironmentMetadata(env *envpkg.Environment) []MetadataItem {
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

func isEmptyConfigValue(value string) bool {
	if value == "" {
		return true
	}
	var v any
	if err := json.Unmarshal([]byte(value), &v); err != nil {
		return false
	}
	switch typed := v.(type) {
	case nil:
		return true
	case string:
		return typed == ""
	case []any:
		return len(typed) == 0
	case map[string]any:
		return len(typed) == 0
	default:
		return false
	}
}

func gcpProjectIDFromValues(values []*envpkg.EnvironmentValue) string {
	for _, v := range values {
		if v.Key == "project_id" && !v.Secret {
			return components.RawValueForDisplay(v.Value)
		}
	}
	return ""
}

func featureBreadcrumbs(tenant *envpkg.Tenant, env *envpkg.Environment, featureName string) []breadcrumb.Crumb {
	envCrumb := breadcrumb.FeatureEnvironment(featureName, tenant.Name, env.Name)
	envCrumb.Icon = components.TenantAvatar(tenant.Name, components.HasTenantLogo(tenant.Name), "18px")
	return []breadcrumb.Crumb{
		breadcrumb.Features(),
		breadcrumb.Feature(featureName),
		envCrumb,
	}
}

func loadFeaturePageData(ctx context.Context, tenantSlug, envName, featureName, activeTab, expandedLogID string, showAllDeploys bool) (*FeaturePage, error) {
	tenant, err := envpkg.GetTenantByName(ctx, tenantSlug)
	if err != nil {
		return nil, err
	}

	env, err := envpkg.GetByName(ctx, tenant.ID, envName)
	if err != nil {
		return nil, err
	}

	feat, err := featureassignment.FeatureForEnvironment(ctx, env.ID, featureName)
	if err != nil {
		return nil, err
	}

	_, disabled, err := featurepkg.FeatureDisabledAt(ctx, env.ID, featureName)
	if err != nil {
		return nil, err
	}

	var disableReason string
	if disabled {
		disableReason = audit.LatestDisableReason(ctx, featureName, env.ID)
	}

	breadcrumbs := featureBreadcrumbs(tenant, env, featureName)

	page := &FeaturePage{
		Breadcrumbs:   breadcrumbs,
		Tenant:        tenant,
		TenantSlug:    tenantSlug,
		Environment:   &Environment{Environment: env, Metadata: getEnvironmentMetadata(env)},
		Feature:       &FeatureDetail{Feature: feat, Enabled: !disabled, DisableReason: disableReason},
		FeatureEnvs:   featureenvs.LoadEnvironments(ctx, feat),
		ActiveTab:     activeTab,
		ExpandedLogID: expandedLogID,
	}

	if !env.Reconcile {
		page.Status = "DISABLED"
	} else if disabled {
		page.Status = "DISABLED"
	} else if status, msg, err := featureassignment.FeatureStatusForEnvironment(ctx, env.ID, featureName); err == nil && status != "" {
		page.Status = status
		page.StatusMessage = msg
	}

	if activeTab == "config" {
		page.Feature.ConfigItems, err = loadFeatureConfigItems(ctx, feat, env.ID)
		if err != nil {
			return nil, err
		}
	}

	page.FeatureLog = loadFeatureLog(ctx, env.ID, feat)

	if activeTab == "assignments" {
		page.Assignments = loadEnvironmentAssignments(ctx, featureName, env.ID)
	}
	if activeTab == "" || activeTab == "status" {
		if dep, err := featureassignment.WinningAssignment(ctx, env.ID, featureName); err == nil {
			page.WinningAssignment = dep
		}
		deployLimit := 11
		if showAllDeploys {
			deployLimit = 100
		}
		page.RecentDeployHistory, _ = featurepkg.ListRecentDeployInstructions(ctx, env.ID, featureName, deployLimit)
		if !showAllDeploys && len(page.RecentDeployHistory) > 10 {
			page.ShowAllDeploys = false // there are more
			page.RecentDeployHistory = page.RecentDeployHistory[:10]
		} else {
			page.ShowAllDeploys = true // already showing everything
		}
		page.DeployLogsByInstruction = map[string][]LogLine{}
		if page.ExpandedLogID != "" {
			for _, di := range page.RecentDeployHistory {
				if di.ID.String() != page.ExpandedLogID {
					continue
				}
				lines, err := featurepkg.LogsGet(ctx, di.ID)
				if err != nil {
					break
				}
				entries := make([]LogLine, 0, len(lines))
				for _, line := range lines {
					entries = append(entries, LogLine{Timestamp: view.FormatTime(line.Timestamp), Message: line.Message})
				}
				page.DeployLogsByInstruction[di.ID.String()] = entries
				break
			}
		}
	}
	switch activeTab {
	case "config":
		entries, err := audit.ListConfigForFeatureInEnvironment(ctx, featureName, env.ID, 10)
		if err != nil {
			return nil, fmt.Errorf("load audit entries: %w", err)
		}
		page.AuditEntries = entries
	default:
		limit := int32(3)
		if activeTab == "audit" {
			limit = 50
		}
		entries, err := audit.ListForFeatureInEnvironment(ctx, featureName, env.ID, limit)
		if err != nil {
			return nil, fmt.Errorf("load audit entries: %w", err)
		}
		page.AuditEntries = entries
	}

	return page, nil
}

func loadFeatureConfigItems(ctx context.Context, feat *featurepkg.Feature, envID uuid.UUID) ([]FeatureConfigItem, error) {
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
			configs = append(configs, &featurepkg.Configuration{
				ID:      uuid.NewSHA1(uuid.Nil, []byte(feat.Name+"|"+key)),
				Key:     key,
				Value:   &value,
				Content: feat.ValuesYAML[key],
				Source:  featurepkg.ConfigSourceHelm,
			})
		}
	}

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].Key < configs[j].Key
	})

	hasComputed := false
	for _, cfg := range configs {
		if cfg.Value != nil && cfg.Value.Computed != nil {
			hasComputed = true
			break
		}
	}

	var rendered map[string]any
	var computedSecrets map[string]bool
	probeFailed := false
	if hasComputed {
		var probeOK bool
		var rerr error
		rendered, computedSecrets, probeOK, rerr = featurepkg.HelmValuesWithSecretTaint(ctx, feat, envID)
		if rerr != nil {
			rendered = nil
		}
		probeFailed = rerr == nil && !probeOK
	}

	globalConfigs, err := featurepkg.ConfigGet(ctx, feat.Name)
	if err != nil {
		return nil, err
	}
	globalByKey := make(map[string][]byte, len(globalConfigs))
	for _, gc := range globalConfigs {
		globalByKey[gc.Key] = gc.Content
	}

	items := make([]FeatureConfigItem, 0, len(configs))
	for _, cfg := range configs {
		item := FeatureConfigItem{
			ID:          cfg.ID.String(),
			Key:         cfg.Key,
			Source:      string(cfg.Source),
			Value:       components.RawValueForDisplay(cfg.Content),
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
				if probeFailed || computedSecrets[cfg.Key] {
					item.IsSecret = true
				}
			}
		}
		if cfg.Source == featurepkg.ConfigSourceEnv {
			if gv, ok := globalByKey[cfg.Key]; ok {
				item.FallbackValue = components.RawValueForDisplay(gv)
			} else if raw, ok := feat.ValuesYAML[cfg.Key]; ok {
				item.FallbackValue = components.RawValueForDisplay(raw)
			}
		}
		items = append(items, item)
	}

	if rendered != nil {
		for i, item := range items {
			if !item.IsComputed || items[i].IsSecret {
				continue
			}
			if item.Source == string(featurepkg.ConfigSourceEnv) {
				continue
			}
			if v, ok := lookupHelmValue(rendered, item.Key); ok {
				items[i].Value = v
			}
		}
	}

	return items, nil
}

func countTemplateRefs(values featurepkg.Values, key string) int {
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

func loadEnvironmentAssignments(ctx context.Context, featureName string, envID uuid.UUID) []EnvAssignmentItem {
	envLabels, err := envpkg.GetLabels(ctx, envID)
	if err != nil {
		return nil
	}

	assignments, err := featureassignment.ListByFeature(ctx, featureName)
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

	for _, dep := range assignments {
		target := dep.TargetLabels
		if !labelsMatch(envLabels, target) {
			continue
		}

		fallbackState := "UNKNOWN"
		if statuses, err := featureassignment.ListFeatureReconcileStatuses(ctx, dep.ID); err == nil {
			for _, s := range statuses {
				if s.EnvironmentID == envID {
					fallbackState = featureassignment.NormalizeStatus(string(s.State))
					break
				}
			}
		}
		status := fallbackState

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

	items := make([]EnvAssignmentItem, 0, len(candidates))
	for _, c := range candidates {
		if len(c.target) == maxLabels {
			items = append(items, EnvAssignmentItem{
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

func loadFeatureLog(ctx context.Context, envID uuid.UUID, feat *featurepkg.Feature) *FeatureLog {
	di, err := featurepkg.GetLatestDeployInstruction(ctx, envID, feat.Name)
	if err != nil {
		return nil
	}
	lines, err := featurepkg.LogsGet(ctx, di.ID)
	if err != nil {
		return nil
	}
	lastDeployed := "never"
	if dep, err := featurepkg.GetLatestDeployedDeployInstruction(ctx, envID, feat.Name); err == nil && dep != nil {
		lastDeployed = view.FormatTime(dep.LastModified)
	}
	ret := &FeatureLog{
		CurrentVersion:       di.FeatureVersion,
		CurrentStatus:        strings.ToUpper(di.Status.String()),
		LastModified:         view.FormatTime(di.LastModified),
		LastModifiedRelative: view.RelativeTime(di.LastModified),
		LastDeployed:         lastDeployed,
	}
	for _, line := range lines {
		ret.CurrentLog = append(ret.CurrentLog, LogLine{Timestamp: view.FormatTime(line.Timestamp), Message: line.Message})
	}
	ret.HelmDiff, _ = featurepkg.HelmValueDiff(ctx, di, feat.SecretKeys())
	return ret
}
