package database

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/pressly/goose/v3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

type Repo interface {
	ConfigCreate(ctx context.Context, c model.NewConfiguration) (*model.Configuration, error)
	ConfigDelete(ctx context.Context, id uuid.UUID) error
	ConfigGet(ctx context.Context, feature string) ([]*model.Configuration, error)
	ConfigGetForEnv(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error)
	ConfigUpdate(ctx context.Context, id uuid.UUID, c model.UpdateConfiguration) (*model.Configuration, error)
	EnvConfig(ctx context.Context, feature string, envID uuid.UUID) ([]*model.Configuration, error)
	EnvironmentCreate(ctx context.Context, p *model.EnvironmentCreate) (*model.Environment, error)
	EnvironmentGet(ctx context.Context, id uuid.UUID) (*model.Environment, error)
	EnvironmentIDByNames(ctx context.Context, partnerName, environmentName string) (uuid.UUID, error)
	EnvironmentUpdate(ctx context.Context, environmentID uuid.UUID, p *model.EnvironmentUpdate) (*model.Environment, error)
	EnvironmentsGet(ctx context.Context, partnerID uuid.UUID) ([]*model.Environment, error)
	FeatureStatesCreateOrUpdate(ctx context.Context, envID uuid.UUID, feature *feature.Feature, enabled bool) (*model.FeatureState, error)
	FeatureStatesGet(ctx context.Context, envID uuid.UUID) ([]*model.FeatureState, error)
	HelmValues(ctx context.Context, feature string, envID uuid.UUID, requiredFields []string) (map[string]any, error)
	Metrics() prometheus.Collector
	PartnerCreate(ctx context.Context, p *model.PartnerCreate) (*model.Partner, error)
	PartnerGet(ctx context.Context, id uuid.UUID) (*model.Partner, error)
	PartnersGet(ctx context.Context) ([]*model.Partner, error)
	PartnerEnvironments(ctx context.Context) ([]*model.PartnerEnvironments, error)
	StatusCreateOrUpdate(ctx context.Context, environmentID uuid.UUID, h *message.Helm) error
	StatusForEnvironment(ctx context.Context, environmentID uuid.UUID) ([]*model.Status, error)
}

type repo struct {
	querier Querier
	db      *sql.DB
	log     *logrus.Entry
	hooks   *Hooks
}

func (r *repo) Metrics() prometheus.Collector {
	return r.hooks.bucket
}

type Querier interface {
	gensql.Querier
	WithTx(tx *sql.Tx) *gensql.Queries
}

func New(db *sql.DB, log *logrus.Entry) Repo {
	return &repo{
		querier: gensql.New(db),
		db:      db,
		log:     log,
	}
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

	return db, err
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
