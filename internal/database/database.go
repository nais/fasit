package database

import (
	"context"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/url"
	"runtime"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	cloudsqlpgx "cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/nais/fasit/internal/ioconvenience"
	"github.com/pressly/goose/v3"
)

type ConnPoolOption func(*pgxpool.Config)

func WithQueryTracer(tracer pgx.QueryTracer) ConnPoolOption {
	return func(cfg *pgxpool.Config) {
		cfg.ConnConfig.Tracer = tracer
	}
}

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

func NewConnPool(ctx context.Context, dbConnDSN string, log *slog.Logger, opts ...ConnPoolOption) (*pgxpool.Pool, io.Closer, error) {
	cloudsql := !strings.Contains(dbConnDSN, "://")

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

	if runtime.NumCPU() < 20 {
		config.MaxConns = 20
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
		d, err := cloudsqlconn.NewDialer(ctx)
		if err != nil {
			return nil, closers, fmt.Errorf("failed to initialize dialer: %w", err)
		}
		closers = append(closers, d.Close)

		config.ConnConfig.DialFunc = func(ctx context.Context, _ string, instance string) (net.Conn, error) {
			return d.Dial(ctx, cloudsqlHost)
		}

		cleanup, err := cloudsqlpgx.RegisterDriver("cloudsql-postgres", cloudsqlconn.WithIAMAuthN())
		if err != nil {
			return nil, closers, err
		}
		closers = append(closers, cleanup)
	}

	for _, opt := range opts {
		opt(config)
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

// gooseLogger adapts *slog.Logger to goose.Logger.
type gooseLogger struct{ log *slog.Logger }

func (g gooseLogger) Fatalf(format string, v ...any) { g.log.Error(fmt.Sprintf(format, v...)) }
func (g gooseLogger) Printf(format string, v ...any) { g.log.Info(fmt.Sprintf(format, v...)) }

func Migrate(pool *pgxpool.Pool, log *slog.Logger) error {
	goose.SetBaseFS(embedMigrations)
	goose.SetLogger(gooseLogger{log: log.With("subsystem", "database-migration")})

	db := stdlib.OpenDBFromPool(pool)
	defer ioconvenience.CloseWithLog(db, log)

	if err := goose.Up(db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
