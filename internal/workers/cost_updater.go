package workers

import (
	"context"
	"time"

	"cloud.google.com/go/bigquery"
	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

const (
	gcpProject = "nais-io"
	query      = `
SELECT
  tenant,
  COALESCE(env, 'management') AS env,
  dato,
  ROUND(SUM(calculated_cost), 2) AS total
FROM
  ` + "`nais-io.nais_billing_regional.cost_breakdown_total_daily_update`" + `
WHERE
  dato >= TIMESTAMP_SUB(CURRENT_DATE(), INTERVAL 5 DAY)
  AND team = 'nais'
GROUP BY
  tenant,
  env,
  dato
ORDER BY
  tenant ASC,
  env ASC,
  dato DESC
`
)

type CostUpdater struct {
	log    logrus.FieldLogger
	repo   database.Repo
	client *bigquery.Client
}

func NewCostUpdater(ctx context.Context, repo database.Repo, log logrus.FieldLogger) (*CostUpdater, error) {
	client, err := bigquery.NewClient(ctx, gcpProject)
	if err != nil {
		return nil, err
	}
	client.Location = "EU"
	it := client.Datasets(ctx)
	for {
		_, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	return &CostUpdater{
		repo:   repo,
		client: client,
		log:    log,
	}, nil
}

func (c *CostUpdater) Run(ctx context.Context, schedule time.Duration) {
	ticker := time.NewTicker(schedule)
	for {
		if err := c.updateCosts(ctx); err != nil {
			c.log.WithError(err).Error("failed to update costs")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// updateCosts will insert the latest cost data into the database
// it will only do so if the current date is newer than the latest date in the database +1 day
// and the time is after 05:00
func (c *CostUpdater) updateCosts(ctx context.Context) error {
	lastDate, err := c.repo.CostLastDate(ctx)
	if err != nil {
		return err
	}

	// Only run if the latest date recorded isn't today, and the time is after 05:00
	if lastDate.Day()+1 == time.Now().Day() || time.Now().Hour() < 5 {
		return nil
	}

	q := c.client.Query(query)
	it, err := q.Read(ctx)
	if err != nil {
		return err
	}

	tenantEnvs, err := c.tenantEnvs(ctx)
	if err != nil {
		return err
	}

	rows := []gensql.CostUpsertParams{}

	type Row struct {
		TenantName string     `bigquery:"tenant"`
		EnvName    string     `bigquery:"env"`
		Date       civil.Date `bigquery:"dato"`
		Cost       float32    `bigquery:"total"`
	}
	for {
		var r Row
		err := it.Next(&r)
		if err == iterator.Done {
			break
		}
		if err != nil {
			if err == context.Canceled {
				return err
			}

			c.log.WithError(err).Error("failed to read row")
			continue
		}

		tenant, ok := tenantEnvs[r.TenantName]
		if !ok {
			c.log.WithField("tenant", r.TenantName).Debug("no tenant found")
			continue
		}

		env, ok := tenant.envs[r.EnvName]
		if !ok {
			c.log.WithField("tenant", r.TenantName).WithField("env", r.EnvName).Debug("no env found")
			continue
		}

		rows = append(rows, gensql.CostUpsertParams{
			EnvID:    env,
			TenantID: tenant.id,
			Date:     pgtype.Date{Time: r.Date.In(time.UTC), Valid: true},
			Cost:     r.Cost,
		})
	}

	if len(rows) == 0 {
		return nil
	}

	return c.repo.CostUpsert(ctx, rows)
}

type tenantEnvMap struct {
	envs map[string]uuid.UUID
	id   uuid.UUID
}

func (c *CostUpdater) tenantEnvs(ctx context.Context) (tenants map[string]tenantEnvMap, err error) {
	te, err := c.repo.TenantEnvironments(ctx, false)
	if err != nil {
		return nil, err
	}

	tenants = make(map[string]tenantEnvMap, len(te))
	for _, t := range te {
		if _, ok := tenants[t.TenantName]; !ok {
			tenants[t.TenantName] = tenantEnvMap{
				envs: map[string]uuid.UUID{},
				id:   t.TenantID,
			}
		}

		tenants[t.TenantName].envs[t.Name] = t.ID
	}

	return tenants, nil
}
