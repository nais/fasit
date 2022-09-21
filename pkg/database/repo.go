package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"regexp"
	"time"

	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/0*.sql
var embedMigrations embed.FS

type Repo interface {
	ConfigRepo
	EnvironmentRepo
	EnvironmentValueRepo
	FeatureStateRepo
	HealthRepo
	KubernetesNodeRepo
	ReleaseStatusRepo
	RolloutRepo
	StatusRepo
	TenantRepo

	Close() error
	Metrics() prometheus.Collector
	WithTx(ctx context.Context) (Repo, *sql.Tx, error)
}

type repo struct {
	querier   Querier
	db        *sql.DB
	log       *logrus.Entry
	hooks     *Hooks
	dbConnDSN string
}

func (r *repo) Metrics() prometheus.Collector {
	return r.hooks.bucket
}

type Querier interface {
	gensql.Querier
	WithTx(tx *sql.Tx) *gensql.Queries
}

func New(db *sql.DB, dbConnDSN string, log *logrus.Entry) Repo {
	return &repo{
		querier:   gensql.New(db),
		db:        db,
		log:       log,
		dbConnDSN: dbConnDSN,
	}
}

func (r *repo) WithTx(ctx context.Context) (Repo, *sql.Tx, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, err
	}
	return &repo{
		querier: r.querier.WithTx(tx),
		db:      r.db,
		log:     r.log,
		hooks:   r.hooks,
	}, tx, nil
}

func NewDB(driver, dbConnDSN string) (*sql.DB, error) {
	// TODO(thokra): Remove the prometheus hook to main
	// hooks := NewHooks()
	// sql.Register("psqlhooked", sqlhooks.Wrap(driver, hooks))

	db, err := sql.Open(driver, dbConnDSN)
	if err != nil {
		return nil, fmt.Errorf("open sql connection: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func Migrate(db *sql.DB, log *logrus.Entry) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(log)
	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
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

func (r *repo) Close() error {
	return r.db.Close()
}
