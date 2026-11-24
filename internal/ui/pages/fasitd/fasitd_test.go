package fasitdpage

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/fasitd"
	"github.com/nais/fasit/internal/fasitd/commandstatus"
	g "maragu.dev/gomponents"
)

func render(t *testing.T, n g.Node) string {
	t.Helper()
	var buf bytes.Buffer
	if err := n.Render(&buf); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func sampleOverview() fasitd.EnvironmentOverview {
	now := time.Now()
	return fasitd.EnvironmentOverview{
		EnvironmentID: uuid.New(),
		Tenant:        "nav",
		Environment:   "dev",
		Sessions: []fasitd.SessionStatus{
			{
				ID: uuid.New(), Instance: "fasitd-abc", Version: "v2", Role: fasitd.RolePrimary,
				ConnectedAt: now, LastSeen: now, LastHeartbeat: now,
				MessagesIn: map[string]int64{"heartbeat": 3},
				Recent:     []fasitd.MessageRecord{{Time: now, Direction: fasitd.DirectionOut, Type: "command", Summary: "kyverno@1.2.3 (d1)"}},
			},
			{ID: uuid.New(), Instance: "fasitd-def", Version: "v1", Role: fasitd.RoleDraining, DrainingSince: now},
		},
		Closed:   []fasitd.ClosedSession{{Instance: "fasitd-old", Version: "v1", ConnectedAt: now.Add(-time.Hour), ClosedAt: now, Drained: true, Reason: "closed by fasitd"}},
		Outcomes: map[commandstatus.Status]int64{commandstatus.Deployed: 4, commandstatus.Undeliverable: 1},
		Releases: fasitd.ReleaseSummary{Count: 5, ReportedAt: now, OnlyNaisd: []string{"naisd"}},
	}
}

func TestOverviewPageShowsEnvironmentSummary(t *testing.T) {
	missing := fasitd.EnvironmentOverview{Tenant: "nav", Environment: "prod"}
	html := render(t, overviewPage([]fasitd.EnvironmentOverview{sampleOverview(), missing}))

	for _, want := range []string{
		`href="/tenants/nav/envs/dev?tab=fasitd"`,
		"1 draining",
		"fasitd-abc",
		"4 deployed",
		"1 undeliverable",
		"1 differ from naisd",
		"never reported",
		"closed by fasitd",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("overview page missing %q", want)
		}
	}
}

func TestEnvironmentContentShowsSessionsAndMessages(t *testing.T) {
	html := render(t, EnvironmentContent(sampleOverview()))

	for _, want := range []string{
		"fasitd-abc",
		"primary",
		"draining",
		"kyverno@1.2.3 (d1)",
		"heartbeat 3",
		"only reported by naisd",
		"fasitd-old",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("environment content missing %q", want)
		}
	}
}
