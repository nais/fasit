package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"embed"
	"fmt"
	"regexp"
	"time"

	"github.com/nais/fasit/pkg/database/gensql"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"github.com/pressly/goose/v3"
	"github.com/qustavo/sqlhooks/v2"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Repo struct {
	querier Querier
	db      *sql.DB
	log     *logrus.Entry
	hooks   *Hooks
}

func (r *Repo) Metrics() prometheus.Collector {
	return r.hooks.bucket
}

type Querier interface {
	gensql.Querier
	WithTx(tx *sql.Tx) *gensql.Queries
}

func New(driver driver.Driver, dbConnDSN string, log *logrus.Entry) (*Repo, error) {
	log.Info("creating new driver")
	hooks := NewHooks()
	sql.Register("psqlhooked", sqlhooks.Wrap(driver, hooks))
	log.Info("registered")

	db, err := sql.Open("psqlhooked", dbConnDSN)
	if err != nil {
		return nil, fmt.Errorf("open sql connection: %w", err)
	}
	log.Info("successfully opened connection")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	log.Info("successfully pinged connection")

	goose.SetBaseFS(embedMigrations)

	if err := goose.Up(db, "migrations"); err != nil {
		return nil, fmt.Errorf("goose up: %w", err)
	}
	log.Info("successfully migrated")

	return &Repo{
		querier: gensql.New(db),
		db:      db,
		log:     log,
		hooks:   hooks,
	}, nil
}

// Hooks satisfies the sqlhook.Hooks interface
type Hooks struct {
	bucket *prometheus.HistogramVec
}

func NewHooks() *Hooks {
	return &Hooks{
		bucket: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "fasit",
			Subsystem: "repo",
			Name:      "query_time",
			Help:      "Query time by name in ms",
			Buckets:   prometheus.ExponentialBuckets(10, 5, 5),
		}, []string{"query"}),
	}
}

type ctxKey string

// Before hook will print the query with it's args and return the context with the timestamp
func (h *Hooks) Before(ctx context.Context, query string, args ...any) (context.Context, error) {
	return context.WithValue(ctx, ctxKey("begin"), time.Now()), nil
}

// After hook will get the timestamp registered on the Before hook and print the elapsed time
func (h *Hooks) After(ctx context.Context, query string, args ...any) (context.Context, error) {
	begin := ctx.Value(ctxKey("begin")).(time.Time)

	name := nameFromQuery(query)
	h.bucket.WithLabelValues(name).Observe(float64(time.Since(begin).Milliseconds()))

	return ctx, nil
}

var sqlNameReg = regexp.MustCompile(`name:\s*([\w\d]+)`)

func nameFromQuery(q string) string {
	submatch := sqlNameReg.FindStringSubmatch(q)
	if len(submatch) > 1 {
		return submatch[1]
	}
	return "Unknown"
}
