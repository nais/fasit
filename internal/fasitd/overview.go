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

// StatusSource provides live session state and persisted summaries for the UI.
type StatusSource interface {
	Overview(ctx context.Context) ([]EnvironmentOverview, error)
	EnvironmentOverview(ctx context.Context, env EnvironmentRef) (EnvironmentOverview, error)
}

type EnvironmentOverview struct {
	EnvironmentRef
	Sessions []SessionStatus
	Closed   []ClosedSession
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
		out[i] = s.liveOverview(EnvironmentRef{ID: env.ID, Tenant: env.TenantName, Environment: env.Name})
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

func (s *Server) EnvironmentOverview(ctx context.Context, env EnvironmentRef) (EnvironmentOverview, error) {
	ov := s.liveOverview(env)
	if err := s.fillPersisted(ctx, &env.ID, map[uuid.UUID]*EnvironmentOverview{env.ID: &ov}); err != nil {
		return EnvironmentOverview{}, err
	}
	return ov, nil
}

func (s *Server) liveOverview(env EnvironmentRef) EnvironmentOverview {
	sessions, closed := s.registry.status(env.key())
	return EnvironmentOverview{
		EnvironmentRef: env,
		Sessions:       sessions,
		Closed:         closed,
		Outcomes:       map[commandstatus.Status]int64{},
		Releases: ReleaseSummary{
			ReportedAt: s.registry.releasesReportedAt(env.ID),
		},
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
			if r.ReportedAt.After(ov.Releases.ReportedAt) {
				ov.Releases.ReportedAt = r.ReportedAt
			}
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
