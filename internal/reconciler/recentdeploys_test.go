package reconciler

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

func TestAggregateRecentDeploys(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	// at returns a time offset by n minutes from base; larger n is newer.
	at := func(n int) time.Time { return base.Add(time.Duration(n) * time.Minute) }

	assignA := uuid.New()
	assignB := uuid.New()
	diidA1, diidA2, diidA3 := uuid.New(), uuid.New(), uuid.New()
	diidB1 := uuid.New()

	row := func(created time.Time, diid uuid.UUID, feature, version, status string, assignment uuid.UUID) reconcilersql.ListRecentDeploysRow {
		return reconcilersql.ListRecentDeploysRow{
			Diid:                diid,
			FeatureName:         feature,
			FeatureVersion:      version,
			Status:              status,
			FeatureAssignmentID: assignment,
			Created:             created,
		}
	}

	// Newest first, as the query returns. Feature "a" v1 rolls out to three
	// environments; diidA1 has two lifecycle rows (latest wins). Feature "b" has
	// the single oldest event, so it sorts last.
	rows := []reconcilersql.ListRecentDeploysRow{
		row(at(50), diidA1, "a", "1", "deployed", assignA),   // latest for diidA1
		row(at(40), diidA2, "a", "1", "failed", assignA),     //
		row(at(30), diidA3, "a", "1", "installing", assignA), //
		row(at(20), diidA1, "a", "1", "installing", assignA), // stale dup of diidA1
		row(at(10), diidB1, "b", "2", "deployed", assignB),   //
	}

	got := aggregateRecentDeploys(rows, 10)

	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2", len(got))
	}

	a := got[0]
	if a.FeatureName != "a" || a.FeatureVersion != "1" {
		t.Fatalf("first group = %s/%s, want a/1", a.FeatureName, a.FeatureVersion)
	}
	if a.Total != 3 {
		t.Errorf("a total = %d, want 3 (one per diid, deduped)", a.Total)
	}
	if a.Deployed != 1 || a.Failed != 1 || a.Pending != 1 {
		t.Errorf("a counts = deployed %d failed %d pending %d, want 1/1/1", a.Deployed, a.Failed, a.Pending)
	}
	if !a.LastDeploy.Equal(at(50)) {
		t.Errorf("a last deploy = %s, want %s", a.LastDeploy, at(50))
	}
	if a.FeatureAssignmentID != assignA {
		t.Errorf("a assignment = %s, want %s", a.FeatureAssignmentID, assignA)
	}

	b := got[1]
	if b.FeatureName != "b" || b.Total != 1 || b.Deployed != 1 {
		t.Errorf("second group = %s total %d deployed %d, want b total 1 deployed 1", b.FeatureName, b.Total, b.Deployed)
	}
}

func TestAggregateRecentDeploysOrdersByMostRecentEvent(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	mk := func(min int, feature string) reconcilersql.ListRecentDeploysRow {
		return reconcilersql.ListRecentDeploysRow{
			Diid:        uuid.New(),
			FeatureName: feature,
			Status:      "deployed",
			Created:     base.Add(time.Duration(min) * time.Minute),
		}
	}
	// newest first overall, but feature "old" has an earlier second event so it
	// must still sort after "new".
	rows := []reconcilersql.ListRecentDeploysRow{
		mk(50, "new"),
		mk(40, "old"),
		mk(35, "new"),
		mk(20, "old"),
	}

	got := aggregateRecentDeploys(rows, 10)
	if len(got) != 2 || got[0].FeatureName != "new" || got[1].FeatureName != "old" {
		t.Fatalf("group order = %v, want [new old]", []string{got[0].FeatureName, got[1].FeatureName})
	}
}

func TestAggregateRecentDeploysLimit(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	var rows []reconcilersql.ListRecentDeploysRow
	for i := 0; i < 5; i++ {
		rows = append(rows, reconcilersql.ListRecentDeploysRow{
			Diid:        uuid.New(),
			FeatureName: string(rune('a' + i)),
			Status:      "deployed",
			Created:     base.Add(time.Duration(-i) * time.Minute),
		})
	}

	got := aggregateRecentDeploys(rows, 2)
	if len(got) != 2 {
		t.Fatalf("got %d groups, want 2 (limited)", len(got))
	}
	if got[0].FeatureName != "a" || got[1].FeatureName != "b" {
		t.Errorf("limited groups = %s,%s, want a,b", got[0].FeatureName, got[1].FeatureName)
	}
}

func TestAggregateRecentDeploysCountsUnknown(t *testing.T) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	rows := []reconcilersql.ListRecentDeploysRow{
		{Diid: uuid.New(), FeatureName: "a", Status: "deployed", Created: base.Add(2 * time.Minute)},
		{Diid: uuid.New(), FeatureName: "a", Status: "weird", Created: base.Add(time.Minute)},
	}
	got := aggregateRecentDeploys(rows, 10)
	if len(got) != 1 {
		t.Fatalf("got %d groups, want 1", len(got))
	}
	g := got[0]
	if g.Total != 2 || g.Deployed != 1 || g.Failed != 0 || g.Pending != 0 {
		t.Errorf("counts = total %d deployed %d failed %d pending %d; unknown status must count toward total only",
			g.Total, g.Deployed, g.Failed, g.Pending)
	}
}
