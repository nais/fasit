package auditview

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/audit"
	g "maragu.dev/gomponents"
)

func renderToString(t *testing.T, node g.Node) string {
	t.Helper()
	var buf strings.Builder
	if err := node.Render(&buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	return buf.String()
}

func TestRedeployShowsFeatureAndEnvironment(t *testing.T) {
	e := &audit.Entry{
		Action:          audit.ActionRedeploy,
		ObjectType:      audit.ObjectTypeFeatureAssignment,
		ObjectID:        "loki",
		TenantName:      "nav",
		EnvironmentName: "dev",
	}

	if got := DisplayAction(e); got != "redeployed" {
		t.Errorf("action = %q, want %q", got, "redeployed")
	}

	rendered := renderToString(t, ResourceLink(e))
	if !strings.Contains(rendered, "loki") {
		t.Errorf("resource link should contain feature name, got %q", rendered)
	}
	if !strings.Contains(rendered, "nav/dev") {
		t.Errorf("resource link should contain environment, got %q", rendered)
	}
}

func TestConfigUpdateShowsOldAndNewValues(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/port",
		Metadata:   json.RawMessage(`{"old":"3000","new":"8080"}`),
	}

	rendered := renderToString(t, ConfigChangeNode(e))
	if !strings.Contains(rendered, "3000") {
		t.Errorf("should show old value, got %q", rendered)
	}
	if !strings.Contains(rendered, "8080") {
		t.Errorf("should show new value, got %q", rendered)
	}
	if !strings.Contains(rendered, "→") {
		t.Errorf("should show arrow between values, got %q", rendered)
	}
}

func TestConfigCreateShowsNewValue(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/port",
		Metadata:   json.RawMessage(`{"new":"\"hello\""}`),
	}

	rendered := renderToString(t, ConfigChangeNode(e))
	if !strings.Contains(rendered, "hello") {
		t.Errorf("should show new value, got %q", rendered)
	}
	if strings.Contains(rendered, "→") {
		t.Errorf("should not show arrow for create, got %q", rendered)
	}
}

func TestConfigDeleteShowsOldValue(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/port",
		Metadata:   json.RawMessage(`{"old":"\"gone\""}`),
	}

	rendered := renderToString(t, ConfigChangeNode(e))
	if !strings.Contains(rendered, "gone") {
		t.Errorf("should show deleted value, got %q", rendered)
	}
	if !strings.Contains(rendered, "val-deleted") {
		t.Errorf("should have strikethrough class, got %q", rendered)
	}
}

func TestSecretConfigDoesNotExposeValues(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/token",
		Metadata:   json.RawMessage(`{"secret":"true"}`),
	}

	rendered := renderToString(t, ConfigChangeNode(e))
	if !strings.Contains(rendered, "(secret)") {
		t.Errorf("should indicate secret, got %q", rendered)
	}
}

func TestLongValuesAreTruncatedWithHoverForFull(t *testing.T) {
	long := `["10.53.140.47","10.53.161.11","10.53.104.43","10.53.104.44"]`
	meta, _ := json.Marshal(map[string]string{"old": long, "new": long + "x"})
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/ips",
		Metadata:   meta,
	}

	rendered := renderToString(t, ConfigChangeNode(e))
	if !strings.Contains(rendered, "…") {
		t.Errorf("should truncate long values, got %q", rendered)
	}
	if !strings.Contains(rendered, "title=") {
		t.Errorf("should have title attribute for hover, got %q", rendered)
	}
}

func TestConfigCellShowsEnvironmentLocation(t *testing.T) {
	envID := uuid.New()
	e := &audit.Entry{
		ObjectType:      audit.ObjectTypeConfiguration,
		ObjectID:        "myapp/port",
		Metadata:        json.RawMessage(`{"old":"3000","new":"8080"}`),
		EnvironmentID:   &envID,
		TenantName:      "test-partner",
		EnvironmentName: "dev",
	}

	rendered := renderToString(t, configCell(e))
	if !strings.Contains(rendered, "in test-partner/dev") {
		t.Errorf("should show tenant/environment location, got %q", rendered)
	}
	if !strings.Contains(rendered, "8080") {
		t.Errorf("should still show the value diff, got %q", rendered)
	}
}

func TestConfigCellShowsGlobalForTenantWideConfig(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/port",
		Metadata:   json.RawMessage(`{"old":"3000","new":"8080"}`),
	}

	rendered := renderToString(t, configCell(e))
	if !strings.Contains(rendered, "global") {
		t.Errorf("should mark a config with no environment as global, got %q", rendered)
	}
}

func TestConfigResourceLinkGoesToConfigTab(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/port",
	}

	href := ResourceHref(e)
	if href != "/features/myapp/config#config-port" {
		t.Errorf("href = %q, want link to config tab with anchor", href)
	}
}

func TestEnvConfigResourceLinkGoesToEnvConfigTab(t *testing.T) {
	envID := uuid.New()
	e := &audit.Entry{
		ObjectType:      audit.ObjectTypeConfiguration,
		ObjectID:        "myapp/port",
		EnvironmentID:   &envID,
		TenantName:      "nav",
		EnvironmentName: "dev",
	}

	href := ResourceHref(e)
	if href != "/tenants/nav/envs/dev/features/myapp/config#config-port" {
		t.Errorf("href = %q, want link to env config tab with anchor", href)
	}
}

func TestAssignmentCreatedShowsVersionAndTarget(t *testing.T) {
	e := &audit.Entry{
		Action:      audit.ActionCreated,
		ObjectType:  audit.ObjectTypeFeatureAssignment,
		ObjectID:    "loki",
		Description: "version 2.0.0",
		Metadata:    json.RawMessage(`{"target":{"team":"nav","env":"dev"}}`),
	}

	desc := Description(e)
	if !strings.Contains(desc, "2.0.0") {
		t.Errorf("should contain version, got %q", desc)
	}
	if !strings.Contains(desc, "env=dev") {
		t.Errorf("should contain target labels, got %q", desc)
	}
}

func TestRedeployDescriptionIsRedundant(t *testing.T) {
	e := &audit.Entry{
		Action:     audit.ActionRedeploy,
		ObjectType: audit.ObjectTypeFeatureAssignment,
		ObjectID:   "loki",
	}

	if !IsDescriptionRedundant(e) {
		t.Error("redeploy description should be redundant (info already in action + resource)")
	}
}

func TestConfigDescriptionIsRedundant(t *testing.T) {
	e := &audit.Entry{
		ObjectType: audit.ObjectTypeConfiguration,
		ObjectID:   "myapp/port",
	}

	if !IsDescriptionRedundant(e) {
		t.Error("config description should be redundant (info already in resource + change node)")
	}
}

func TestNonConfigNonRedeployDescriptionIsShown(t *testing.T) {
	e := &audit.Entry{
		Action:     audit.ActionCreated,
		ObjectType: audit.ObjectTypeFeatureAssignment,
		ObjectID:   "loki",
	}

	if IsDescriptionRedundant(e) {
		t.Error("assignment created description should not be redundant")
	}
}
