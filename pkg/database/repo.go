package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net"
	"net/url"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	cloudsqlpgx "cloud.google.com/go/cloudsqlconn/postgres/pgxv4"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type (
	ctxKey     string
	closeFuncs []func() error
)

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

type TXFunc func(repo Repo) error

type Repo interface {
	AuditRepo
	AutoInstallsRepo
	ConfigRepo
	DeployInstructionRepo
	EnvironmentRepo
	EnvironmentValueRepo
	FeatureDataRepo
	FeaturesRepo
	FeatureStateRepo
	HealthRepo
	KubernetesNodeRepo
	LogRepo
	ReleaseStatusRepo
	RolloutRepo
	StatusRepo
	TenantRepo
	WarningRepo

	Transaction

	Close()
	Metrics(meter metric.Meter) error
	WithTx(ctx context.Context) (Repo, pgx.Tx, error)
}

type Transaction interface {
	TxFunc(ctx context.Context, fn TXFunc) error
}

type repo struct {
	querier     Querier
	db          *pgxpool.Pool
	log         *logrus.Entry
	oldFeatures *feature.Manager

	auditErrorCount metric.Int64Counter
}

func (r *repo) Metrics(meter metric.Meter) (err error) {
	r.auditErrorCount, err = meter.Int64Counter("audit_errors", metric.WithDescription("Number of audit errors"))
	if err != nil {
		return fmt.Errorf("failed to create audit_errors counter: %w", err)
	}

	return nil
}

type Querier interface {
	gensql.Querier
	WithTx(tx pgx.Tx) *gensql.Queries
}

func New(db *pgxpool.Pool, mgr *feature.Manager, log *logrus.Entry) Repo {
	return &repo{
		querier:     gensql.New(db),
		db:          db,
		log:         log,
		oldFeatures: mgr,
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
	}, tx, nil
}

func (r *repo) TxFunc(ctx context.Context, fn TXFunc) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		return fn(&repo{
			querier: r.querier.WithTx(tx),
			db:      r.db,
			log:     r.log,
		})
	})
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
	conn, err := pgxpool.NewWithConfig(context.Background(), config)
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

func (r *repo) Close() {
	r.db.Close()
}
