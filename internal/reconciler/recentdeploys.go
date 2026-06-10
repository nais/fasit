package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
)

// recentDeployScanRows bounds how many recent deploy_log rows we pull to
// aggregate. A single deploy contributes a few rows (sent/installing/terminal),
// so this comfortably covers far more than the handful of feature versions we
// surface.
const recentDeployScanRows = 1000

// RecentDeploy is a feature version's rollout across environments, rolled up
// from the deploy log: how many of its deploy instructions are deployed, failed
// or in progress, and when the most recent deploy event happened.
type RecentDeploy struct {
	FeatureName         string
	FeatureVersion      string
	Total               int
	Deployed            int
	Failed              int
	Pending             int
	LastDeploy          time.Time
	FeatureAssignmentID uuid.UUID
}

// ListRecentDeploys returns the most recently active feature versions, each
// rolled up across its environments, newest first, limited to groups rows.
func ListRecentDeploys(ctx context.Context, groups int) ([]RecentDeploy, error) {
	rows, err := querier(ctx).ListRecentDeploys(ctx, recentDeployScanRows)
	if err != nil {
		return nil, fmt.Errorf("list recent deploys: %w", err)
	}
	return aggregateRecentDeploys(rows, groups), nil
}

// aggregateRecentDeploys rolls up deploy_log rows (newest first) into one entry
// per feature version. It keeps only the latest status per deploy instruction
// (diid) and orders groups by their most recent deploy event.
func aggregateRecentDeploys(rows []reconcilersql.ListRecentDeploysRow, groups int) []RecentDeploy {
	type key struct{ feature, version string }
	byKey := make(map[key]*RecentDeploy)
	var order []key
	seenDiid := make(map[uuid.UUID]struct{})

	// rows are newest first; the first row we see for a diid is its latest
	// status, and the first row for a feature version is its most recent event.
	for _, r := range rows {
		if _, ok := seenDiid[r.Diid]; ok {
			continue
		}
		seenDiid[r.Diid] = struct{}{}

		k := key{r.FeatureName, r.FeatureVersion}
		g, ok := byKey[k]
		if !ok {
			g = &RecentDeploy{
				FeatureName:         r.FeatureName,
				FeatureVersion:      r.FeatureVersion,
				LastDeploy:          r.Created,
				FeatureAssignmentID: r.FeatureAssignmentID,
			}
			byKey[k] = g
			order = append(order, k)
		}
		g.Total++
		switch NormalizeStatus(r.Status) {
		case "DEPLOYED":
			g.Deployed++
		case "FAILED":
			g.Failed++
		case "SENT", "INSTALLING":
			g.Pending++
		}
	}

	if groups > len(order) {
		groups = len(order)
	}
	result := make([]RecentDeploy, groups)
	for i := 0; i < groups; i++ {
		result[i] = *byKey[order[i]]
	}
	return result
}
