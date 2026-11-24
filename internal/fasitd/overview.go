package fasitd

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	envpkg "github.com/nais/fasit/internal/environment"
	"github.com/nais/fasit/internal/fasitd/commandstatus"
	"github.com/nais/fasit/internal/fasitd/fasitdsql"
	"github.com/nais/fasit/internal/fasitd/protogen"
)

// OutcomeWindow is how far back command outcome counts reach.
const OutcomeWindow = 24 * time.Hour

type EnvironmentOverview struct {
	EnvironmentID uuid.UUID
	Tenant        string
	Environment   string
	Sessions      []SessionStatus
	Closed        []ClosedSession
	// Outcomes counts commands created within OutcomeWindow by latest status.
	Outcomes map[commandstatus.Status]int64
	Releases ReleaseSummary
}

func (o EnvironmentOverview) Primary() (SessionStatus, bool) {
	for _, s := range o.Sessions {
		if s.Role == RolePrimary {
			return s, true
		}
	}
	return SessionStatus{}, false
}

// ReleaseSummary compares the release inventory last reported by fasitd with
// the one reported by naisd for the same environment.
type ReleaseSummary struct {
	Count      int64
	ReportedAt time.Time
	OnlyFasitd []string
	OnlyNaisd  []string
	Mismatched []ReleaseMismatch
}

func (r ReleaseSummary) Reported() bool { return !r.ReportedAt.IsZero() }

func (r ReleaseSummary) Differences() int {
	return len(r.OnlyFasitd) + len(r.OnlyNaisd) + len(r.Mismatched)
}

type ReleaseMismatch struct {
	Name          string
	FasitdVersion string
	FasitdStatus  string
	NaisdVersion  string
	NaisdStatus   string
}

// Overview returns the fasitd status of every environment. ctx must carry the
// environment querier (see contextloader).
func (s *Server) Overview(ctx context.Context) ([]EnvironmentOverview, error) {
	envs, err := envpkg.ListTenantEnvironments(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("list environments: %w", err)
	}
	byID := make(map[uuid.UUID]*EnvironmentOverview, len(envs))
	out := make([]EnvironmentOverview, len(envs))
	for i, env := range envs {
		out[i] = s.liveOverview(env.ID, env.TenantName, env.Name)
		byID[env.ID] = &out[i]
	}
	if err := s.fillPersisted(ctx, nil, byID); err != nil {
		return nil, err
	}
	slices.SortFunc(out, func(a, b EnvironmentOverview) int {
		return cmp.Or(cmp.Compare(a.Tenant, b.Tenant), cmp.Compare(a.Environment, b.Environment))
	})
	return out, nil
}

func (s *Server) EnvironmentOverview(ctx context.Context, envID uuid.UUID, tenant, environment string) (EnvironmentOverview, error) {
	ov := s.liveOverview(envID, tenant, environment)
	if err := s.fillPersisted(ctx, &envID, map[uuid.UUID]*EnvironmentOverview{envID: &ov}); err != nil {
		return EnvironmentOverview{}, err
	}
	return ov, nil
}

func (s *Server) liveOverview(envID uuid.UUID, tenant, environment string) EnvironmentOverview {
	sessions, closed := s.registry.status(keyFor(tenant, environment))
	return EnvironmentOverview{
		EnvironmentID: envID,
		Tenant:        tenant,
		Environment:   environment,
		Sessions:      sessions,
		Closed:        closed,
		Outcomes:      map[commandstatus.Status]int64{},
	}
}

func (s *Server) fillPersisted(ctx context.Context, envID *uuid.UUID, byID map[uuid.UUID]*EnvironmentOverview) error {
	outcomes, err := s.querier.ListCommandOutcomeCounts(ctx, fasitdsql.ListCommandOutcomeCountsParams{
		Since:         time.Now().Add(-OutcomeWindow),
		EnvironmentID: envID,
	})
	if err != nil {
		return fmt.Errorf("list command outcomes: %w", err)
	}
	for _, o := range outcomes {
		if ov, ok := byID[o.EnvironmentID]; ok {
			ov.Outcomes[commandstatus.Status(o.Status)] += o.Count
		}
	}

	reports, err := s.querier.ListReleaseInventoryReports(ctx, envID)
	if err != nil {
		return fmt.Errorf("list release inventory reports: %w", err)
	}
	for _, r := range reports {
		if ov, ok := byID[r.EnvironmentID]; ok {
			ov.Releases.Count = r.Releases
			ov.Releases.ReportedAt = r.ReportedAt
		}
	}

	rows, err := s.querier.ListReleaseComparison(ctx, envID)
	if err != nil {
		return fmt.Errorf("compare release inventories: %w", err)
	}
	for _, row := range rows {
		ov, ok := byID[row.EnvironmentID]
		if !ok || !ov.Releases.Reported() {
			continue
		}
		addReleaseDifference(&ov.Releases, row)
	}
	return nil
}

func addReleaseDifference(sum *ReleaseSummary, row fasitdsql.ListReleaseComparisonRow) {
	switch {
	case row.NaisdVersion == nil:
		sum.OnlyFasitd = append(sum.OnlyFasitd, row.Feature)
	case row.FasitdVersion == nil:
		sum.OnlyNaisd = append(sum.OnlyNaisd, row.Feature)
	case *row.FasitdVersion != *row.NaisdVersion || deref(row.FasitdStatus) != deref(row.NaisdStatus):
		sum.Mismatched = append(sum.Mismatched, ReleaseMismatch{
			Name:          row.Feature,
			FasitdVersion: *row.FasitdVersion,
			FasitdStatus:  deref(row.FasitdStatus),
			NaisdVersion:  *row.NaisdVersion,
			NaisdStatus:   deref(row.NaisdStatus),
		})
	}
}

func (s *Server) describeAgentMessage(sess *session, msg *protogen.AgentMessage) (typ, summary string) {
	switch m := msg.GetMessage().(type) {
	case *protogen.AgentMessage_Ack:
		return "ack", sess.stats.commandLabel(m.Ack.GetDiid(), false)
	case *protogen.AgentMessage_Status:
		summary = m.Status.GetStatus() + " " + sess.stats.commandLabel(m.Status.GetDiid(), true)
		if e := m.Status.GetError(); e != "" {
			summary += ": " + e
		}
		return "status", summary
	case *protogen.AgentMessage_Logs:
		return "logs", fmt.Sprintf("%d lines for %s", len(m.Logs.GetLines()), sess.stats.commandLabel(m.Logs.GetDiid(), false))
	case *protogen.AgentMessage_Releases:
		summary = fmt.Sprintf("%d releases", len(m.Releases.GetReleases()))
		if !s.registry.isPrimary(sess) {
			summary += " (ignored, not primary)"
		}
		return "releases", summary
	case *protogen.AgentMessage_Draining:
		return "draining", ""
	case *protogen.AgentMessage_Heartbeat:
		return "heartbeat", ""
	case *protogen.AgentMessage_Register:
		return "register", "duplicate"
	default:
		return "unknown", ""
	}
}

func commandLabel(cmd *protogen.Command) string {
	if cmd.GetUninstall() {
		return "uninstall " + cmd.GetName()
	}
	return cmd.GetName() + "@" + cmd.GetVersion()
}

func commandSummary(cmd *protogen.Command) string {
	return commandLabel(cmd) + " (" + cmd.GetDiid() + ")"
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
