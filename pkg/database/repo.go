package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	cloudsqlpgx "cloud.google.com/go/cloudsqlconn/postgres/pgxv4"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/pressly/goose/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

type closeFuncs []func() error

func (c closeFuncs) Close() error {
	var err error
	for _, f := range c {
		if e := f(); e != nil {
			err = e
		}
	}
	return err
}

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

	Close()
	Metrics() prometheus.Collector
	WithTx(ctx context.Context) (Repo, pgx.Tx, error)
}

type repo struct {
	querier Querier
	db      *pgxpool.Pool
	log     *logrus.Entry
	hooks   *Hooks
}

func (r *repo) Metrics() prometheus.Collector {
	return r.hooks.bucket
}

type Querier interface {
	gensql.Querier
	WithTx(tx pgx.Tx) *gensql.Queries
}

func New(db *pgxpool.Pool, log *logrus.Entry) Repo {
	return &repo{
		querier: gensql.New(db),
		db:      db,
		log:     log,
	}
}

func (r *repo) WithTx(ctx context.Context) (Repo, pgx.Tx, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{})
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

func NewDB(ctx context.Context, dbConnDSN string, cloudsql bool) (*pgxpool.Pool, closeFuncs, error) {
	cloudsqlHost := ""
	if cloudsql {
		vals, err := url.ParseQuery(strings.ReplaceAll(dbConnDSN, " ", "&"))
		if err != nil {
			return nil, nil, err
		}
		cloudsqlHost = vals.Get("host")
		delete(vals, "host")
		dbConnDSN = strings.ReplaceAll(vals.Encode(), "&", " ")
	}

	config, err := pgxpool.ParseConfig(dbConnDSN)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse pgx config: %w", err)
	}

	closers := closeFuncs{}

	if cloudsql {
		// Create a new dialer with any options
		d, err := cloudsqlconn.NewDialer(ctx)
		if err != nil {
			return nil, closers, fmt.Errorf("failed to initialize dialer: %w", err)
		}
		closers = append(closers, d.Close)

		// Tell the driver to use the Cloud SQL Go Connector to create connections
		config.ConnConfig.DialFunc = func(ctx context.Context, _ string, instance string) (net.Conn, error) {
			return d.Dial(ctx, cloudsqlHost)
		}

		cleanup, err := cloudsqlpgx.RegisterDriver("cloudsql-postgres", cloudsqlconn.WithIAMAuthN())
		if err != nil {
			return nil, closers, err
		}
		closers = append(closers, cleanup)
	}

	// Interact with the dirver directly as you normally would
	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		return nil, closers, fmt.Errorf("failed to connect: %w", err)
	}
	return conn, closers, nil
}

func Migrate(driver, dsn string, log *logrus.Entry) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(log)

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return err
	}
	defer db.Close()
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

func (r *repo) Close() {
	r.db.Close()
}
