package database

import (
	"context"
	"embed"
	"fmt"
	"io"
	"net"
	"net/url"
	"runtime"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	cloudsqlpgx "cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/pressly/goose/v3"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
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

type TXFunc func(repo Repo) error

type Repo interface {
	// Move to audit pkg
	AuditRepo
	// Leave alone
	ClusterUpgraderRepo
	// Move to cost pkg
	CostRepo
	// Can possibly be moved, must analyze usage
	DeployInstructionRepo
	// Used in grpc so need context setup there
	EnvironmentRepo
	// Used in grpc so need context setup there
	EnvironmentValueRepo
	// Leave alone
	KubernetesNodeRepo
	// Move to suitable pkg
	LogRepo
	// Can possibly be moved, must analyze usage
	ReleaseStatusRepo
	// Can be moved but is also heavily used in listeners etc
	RolloutRepo
	// Used in grpc so need context setup there
	TenantRepo
	// Can possibly be moved, must analyze usage
	WarningRepo

	Transaction

	Close()
	WithTx(ctx context.Context) (Repo, pgx.Tx, error)
}

type Transaction interface {
	TxFunc(ctx context.Context, fn TXFunc) error
}

type repo struct {
	querier Querier
	db      *pgxpool.Pool
	log     logrus.FieldLogger

	auditErrorCount metric.Int64Counter
}

type Querier interface {
	gensql.Querier
	WithTx(tx pgx.Tx) *gensql.Queries
}

func NewRepo(pool *pgxpool.Pool, log logrus.FieldLogger) Repo {
	return &repo{
		querier: gensql.New(pool),
		db:      pool,
		log:     log.WithField("subsystem", "database-repo"),
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

func NewConnPool(ctx context.Context, dbConnDSN string, log logrus.FieldLogger) (*pgxpool.Pool, io.Closer, error) {
	cloudsql := !strings.Contains(dbConnDSN, "://")

	if runtime.NumCPU() < 5 {
		dbConnDSN = dbConnDSN + " pool_max_conns=5"
	}

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

	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		types, err := conn.LoadTypes(ctx, []string{"environment_kind", "_environment_kind"})
		if err != nil {
			return fmt.Errorf("failed to load types: %w", err)
		}

		conn.TypeMap().RegisterTypes(types)
		return nil
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

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, closers, fmt.Errorf("failed to connect: %w", err)
	}

	if err := Migrate(pool, log); err != nil {
		return nil, closers, fmt.Errorf("error migrating database: %w", err)
	}

	return pool, closers, nil
}

func Migrate(pool *pgxpool.Pool, log logrus.FieldLogger) error {
	log = log.WithField("subsystem", "database-migration")

	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(log)

	db := stdlib.OpenDBFromPool(pool)
	defer ioconvenience.CloseWithLog(db, log)

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

func (r *repo) Close() {
	r.db.Close()
}
