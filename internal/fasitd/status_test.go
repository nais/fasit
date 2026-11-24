package fasitd

import (
	"fmt"
	"testing"
	"time"

	"github.com/nais/fasit/internal/fasitd/fasitdsql"
)

func TestSessionStatsRecentIsNewestFirstAndBounded(t *testing.T) {
	st := newSessionStats("pod-a", time.Unix(0, 0))
	total := recentMessagesPerSession + 7
	for i := range total {
		st.recordIn("status", fmt.Sprint(i), time.Unix(int64(i), 0))
	}
	st.recordIn("heartbeat", "", time.Unix(int64(total), 0))

	var out SessionStatus
	st.fill(&out)

	if len(out.Recent) != recentMessagesPerSession {
		t.Fatalf("recent = %d, want %d", len(out.Recent), recentMessagesPerSession)
	}
	if got, want := out.Recent[0].Summary, fmt.Sprint(total-1); got != want {
		t.Errorf("newest = %q, want %q (heartbeats must not be listed)", got, want)
	}
	if got, want := out.Recent[len(out.Recent)-1].Summary, fmt.Sprint(total-recentMessagesPerSession); got != want {
		t.Errorf("oldest = %q, want %q", got, want)
	}
	if out.MessagesIn["status"] != int64(total) || out.MessagesIn["heartbeat"] != 1 {
		t.Errorf("counters = %v", out.MessagesIn)
	}
	if !out.LastHeartbeat.Equal(time.Unix(int64(total), 0)) {
		t.Errorf("last heartbeat = %v", out.LastHeartbeat)
	}
}

func TestSessionStatsDroppedDoesNotIncrementMessagesOut(t *testing.T) {
	st := newSessionStats("pod-a", time.Unix(0, 0))
	st.recordDropped("undeliverable", "kyverno@1 (d1)", time.Unix(1, 0))

	var out SessionStatus
	st.fill(&out)

	if len(out.MessagesOut) != 0 {
		t.Errorf("MessagesOut = %v, want empty for dropped command", out.MessagesOut)
	}
	if len(out.Recent) != 1 || out.Recent[0].Direction != DirectionDropped {
		t.Errorf("Recent = %+v, want single dropped record", out.Recent)
	}
}

func TestRegistryStatusRolesAndClosedHistory(t *testing.T) {
	r := newRegistry()
	key := keyFor("nav", "prod")
	now := time.Now()

	primary := newSession("nav", "prod")
	primary.stats = newSessionStats("a", now)
	standby := newSession("nav", "prod")
	standby.stats = newSessionStats("b", now)
	draining := newSession("nav", "prod")
	draining.stats = newSessionStats("c", now)

	for _, s := range []*session{draining, primary, standby} {
		r.track(s)
	}
	r.add(primary)
	r.add(standby)

	live, _ := r.status(key)
	got := make([]string, len(live))
	for i, s := range live {
		got[i] = s.Instance + "=" + string(s.Role)
	}
	want := []string{"a=primary", "b=standby", "c=draining"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("sessions = %v, want %v", got, want)
	}

	draining.stats.markDraining(now)
	for i := range recentClosedPerEnvironment + 2 {
		s := newSession("nav", "prod")
		s.stats = newSessionStats(fmt.Sprint(i), now)
		r.track(s)
		r.untrack(s, "gone", now.Add(time.Duration(i)*time.Second))
	}
	r.untrack(draining, "closed by fasitd", now.Add(time.Hour))

	live, closed := r.status(key)
	if len(live) != 2 {
		t.Errorf("live sessions after untrack = %d, want 2", len(live))
	}
	if len(closed) != recentClosedPerEnvironment {
		t.Fatalf("closed = %d, want %d", len(closed), recentClosedPerEnvironment)
	}
	if closed[0].Instance != "c" || !closed[0].Drained {
		t.Errorf("newest closed = %+v, want drained instance c", closed[0])
	}
}

func TestAddReleaseDifference(t *testing.T) {
	v := func(s string) *string { return &s }
	tests := map[string]struct {
		row        fasitdsql.ListReleaseComparisonRow
		onlyFasitd int
		onlyNaisd  int
		mismatched int
	}{
		"equal": {
			row: fasitdsql.ListReleaseComparisonRow{Feature: "x", FasitdVersion: v("1"), FasitdStatus: v("deployed"), NaisdVersion: v("1"), NaisdStatus: v("deployed")},
		},
		"only fasitd": {
			row:        fasitdsql.ListReleaseComparisonRow{Feature: "x", FasitdVersion: v("1"), FasitdStatus: v("deployed")},
			onlyFasitd: 1,
		},
		"only naisd": {
			row:       fasitdsql.ListReleaseComparisonRow{Feature: "x", NaisdVersion: v("1"), NaisdStatus: v("deployed")},
			onlyNaisd: 1,
		},
		"version differs": {
			row:        fasitdsql.ListReleaseComparisonRow{Feature: "x", FasitdVersion: v("1"), FasitdStatus: v("deployed"), NaisdVersion: v("2"), NaisdStatus: v("deployed")},
			mismatched: 1,
		},
		"status differs": {
			row:        fasitdsql.ListReleaseComparisonRow{Feature: "x", FasitdVersion: v("1"), FasitdStatus: v("failed"), NaisdVersion: v("1"), NaisdStatus: v("deployed")},
			mismatched: 1,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var sum ReleaseSummary
			addReleaseDifference(&sum, tc.row)
			if len(sum.OnlyFasitd) != tc.onlyFasitd || len(sum.OnlyNaisd) != tc.onlyNaisd || len(sum.Mismatched) != tc.mismatched {
				t.Errorf("summary = %+v", sum)
			}
		})
	}
}
