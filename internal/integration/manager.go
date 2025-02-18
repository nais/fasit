package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"testing"
	"time"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/notifier"
	"github.com/nais/fasit/internal/graph"
	"github.com/nais/fasit/internal/graph/graphgen"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisd"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/nais/fasit/internal/workers"
	testmanager "github.com/nais/tester/lua"
	"github.com/nais/tester/lua/runner"
	"github.com/nais/tester/lua/spec"
	"github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	lua "github.com/yuin/gopher-lua"
	"go.opentelemetry.io/otel/metric/noop"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
)

type ctxKey int

const (
	poolKey ctxKey = iota
	reconcilerKey
	naisdKey
)

func TestRunner(ctx context.Context, skipSetup bool) (*testmanager.Manager, error) {
	mgr, err := testmanager.New(newConfig, newManager(ctx, skipSetup), &runner.GQL{}, &runner.SQL{}, &runner.PubSub{})
	if err != nil {
		return nil, err
	}

	mgr.AddHelper(&spec.Function{
		Name: "CreateTenant",
		Doc:  "Create a tenant and return its ID",
		Args: []spec.Argument{
			{
				Name: "name",
				Type: []spec.ArgumentType{spec.ArgumentTypeString},
				Doc:  "Name of the tenant",
			},
			{
				Name: "ci?",
				Type: []spec.ArgumentType{spec.ArgumentTypeBoolean},
				Doc:  "Whether the tenant is a CI tenant",
			},
		},
		Returns: []spec.ArgumentType{spec.ArgumentTypeString},
		Func: func(L *lua.LState) int {
			name := L.CheckString(1)
			ci := L.OptBool(2, false)

			pool := L.Context().Value(poolKey).(*pgxpool.Pool)
			repo := database.New(pool, logrus.New())

			tenant, err := repo.TenantCreate(ctx, &model.TenantCreate{
				Name: name,
			})
			if err != nil {
				L.RaiseError("failed to create tenant: %s", err)
			}

			if ci {
				err := pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
					_, err := c.Exec(ctx, `UPDATE tenants SET ci = true WHERE id = $1`, tenant.ID)
					return err
				})
				if err != nil {
					L.RaiseError("failed to make tenant ci: %s", err)
				}
			}

			L.Push(lua.LString(tenant.ID.String()))
			return 1
		},
	})

	mgr.AddHelper(&spec.Function{
		Name: "CreateEnvironment",
		Doc:  "Create an environment and return its ID",
		Args: []spec.Argument{
			{
				Name: "tenant_id",
				Type: []spec.ArgumentType{spec.ArgumentTypeString},
				Doc:  "ID of the tenant",
			},
			{
				Name: "name",
				Type: []spec.ArgumentType{spec.ArgumentTypeString},
				Doc:  "Whether the tenant is a CI tenant",
			},
			{
				Name: "kind",
				Type: []spec.ArgumentType{spec.ArgumentTypeString},
				Doc:  "Kind of the environment",
			},
			{
				Name: "unhealthy?",
				Type: []spec.ArgumentType{spec.ArgumentTypeBoolean},
				Doc:  "Make the environment unhealthy, preventing reconciliation",
			},
			{
				Name: "ci?",
				Type: []spec.ArgumentType{spec.ArgumentTypeBoolean},
				Doc:  "Whether the environment is a CI environment",
			},
		},
		Returns: []spec.ArgumentType{spec.ArgumentTypeString},
		Func: func(L *lua.LState) int {
			tenantIDStr := L.CheckString(1)
			name := L.CheckString(2)
			kindStr := L.CheckString(3)
			unhealthy := L.OptBool(4, false)
			ci := L.OptBool(5, false)

			pool := L.Context().Value(poolKey).(*pgxpool.Pool)
			repo := database.New(pool, logrus.New())

			tenantID, err := uuid.Parse(tenantIDStr)
			if err != nil {
				L.RaiseError("failed to parse tenant ID: %s", err)
			}

			tenant, err := repo.TenantGet(ctx, tenantID)
			if err != nil {
				L.RaiseError("failed to get tenant: %s", err)
			}

			var kind model.EnvironmentKind
			if err := kind.UnmarshalGQL(kindStr); err != nil {
				L.RaiseError("failed to parse environment kind: %s", err)
			}

			env, err := repo.EnvironmentCreate(ctx, &model.EnvironmentCreate{
				Name:     name,
				Kind:     kind,
				TenantID: tenant.ID,
			})
			if err != nil {
				L.RaiseError("failed to create environment: %s", err)
			}

			if ci {
				err := pool.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
					_, err := c.Exec(ctx, `UPDATE environments SET ci = true WHERE id = $1`, env.ID)
					return err
				})
				if err != nil {
					L.RaiseError("failed to make environment ci: %s", err)
				}
			}

			if !unhealthy {
				err := repo.HealthStatusCreateOrUpdate(ctx, env.ID, &message.Health{
					ReportedAt: time.Now(),
				})
				if err != nil {
					L.RaiseError("failed to create health status: %s", err)
				}
			}

			naisd := L.Context().Value(naisdKey).(*naisdRunner)
			if err := naisd.configureEnv(ctx, repo, tenant.Name, env.Name); err != nil {
				L.RaiseError("failed to configure environment: %s", err)
			}

			L.Push(lua.LString(env.ID.String()))
			return 1
		},
	})

	mgr.AddHelper(&spec.Function{
		Name: "Reconcile",
		Doc:  "Reconcile all environments",
		Func: func(L *lua.LState) int {
			reconciler := L.Context().Value(reconcilerKey).(*workers.Reconciler)

			if err := reconciler.Reconcile(ctx); err != nil {
				L.RaiseError("failed to reconcile: %s", err)
			}

			time.Sleep(100 * time.Millisecond)

			return 0
		},
	})

	return mgr, nil
}

func newManager(ctx context.Context, skipSetup bool) testmanager.SetupFunc {
	if skipSetup {
		return func(ctx context.Context, _ string, _ any) (retCtx context.Context, runners []spec.Runner, close func(), err error) {
			return ctx, nil, func() {}, nil
		}
	}

	container, connStr, err := startPostgresql(ctx)
	if err != nil {
		panic(err)
	}

	return func(ctx context.Context, dir string, configInput any) (context.Context, []spec.Runner, func(), error) {
		ctx, done := context.WithCancel(ctx)
		cleanups := []func(){}

		pool, cleanup, err := newDB(ctx, container, connStr)
		if err != nil {
			done()
			return ctx, nil, nil, err
		}
		cleanups = append(cleanups, cleanup)

		log := logrus.New()
		log.Out = io.Discard

		if testing.Verbose() {
			log.Out = os.Stdout
			log.Level = logrus.DebugLevel
		}

		topic := newPubsubRunner()
		// gqlRunner, err := newGQLRunner(ctx, config, pool, topic)
		// if err != nil {
		// 	done()
		// 	return nil, nil, err
		// }

		runners := []spec.Runner{
			// gqlRunner,
			newGQLRunner(pool),
			runner.NewSQLRunner(pool),
			topic,
		}

		ctx = context.WithValue(ctx, poolKey, pool)

		db := database.New(pool, log)
		naisdRunner, close, err := newNaisd(ctx, db)
		if err != nil {
			done()
			return ctx, nil, nil, err
		}

		if err := naisdRunner.start(ctx, db); err != nil {
			done()
			return ctx, nil, nil, err
		}

		cp := func(topicID string, log *logrus.Entry) workers.Publisher {
			p, ok := naisdRunner.reconcilerPublishers[topicID]
			if !ok {
				panic(fmt.Sprintf("no publisher for topic %q", topicID))
			}
			return p
		}

		notifierService := notifier.New(pool, logrus.NewEntry(log))
		reconciler, err := workers.NewReconciler(db, cp, notifierService, noop.NewMeterProvider().Meter(""), logrus.NewEntry(log))
		if err != nil {
			done()
			return ctx, nil, nil, err
		}

		ctx = context.WithValue(ctx, reconcilerKey, reconciler)
		ctx = context.WithValue(ctx, naisdKey, naisdRunner)

		cleanups = append(cleanups, close)

		return ctx, runners, func() {
			for _, cleanup := range cleanups {
				cleanup()
			}
			done()
		}, nil
	}
}

func newGQLRunner(pool *pgxpool.Pool) spec.Runner {
	log := logrus.New()
	log.Out = io.Discard

	resolver := &graph.Resolver{
		Repo: database.New(pool, log),
		Log:  logrus.NewEntry(log),
	}

	newServer := func(es graphql.ExecutableSchema) *handler.Server {
		srv := handler.New(es)
		srv.AddTransport(transport.SSE{})
		srv.AddTransport(transport.GET{})
		srv.AddTransport(transport.POST{})

		return srv
	}

	srv := newServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))

	return runner.NewGQLRunner(srv)
}

func startPostgresql(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	container, err := postgres.Run(ctx, "docker.io/postgres:16-alpine",
		postgres.WithDatabase("example"),
		postgres.WithUsername("example"),
		postgres.WithPassword("example"),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2),
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to start container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, "", fmt.Errorf("failed to get connection string: %w", err)
	}

	logr := logrus.New()
	logr.Out = io.Discard
	pool, close, err := database.NewDB(ctx, connStr, false)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create pool: %w", err)
	}

	if err := database.Migrate("pgx", connStr, logr); err != nil {
		return nil, "", fmt.Errorf("failed to migrate: %w", err)
	}

	pool.Close()
	if err := close.Close(); err != nil {
		return nil, "", fmt.Errorf("failed to close pool: %w", err)
	}

	if err := container.Snapshot(ctx, postgres.WithSnapshotName("migrated")); err != nil {
		return nil, "", fmt.Errorf("failed to snapshot: %w", err)
	}

	return container, connStr, nil
}

func newDB(ctx context.Context, container *postgres.PostgresContainer, connStr string) (*pgxpool.Pool, func(), error) {
	logr := logrus.New()
	logr.Out = io.Discard

	pool, close, err := database.NewDB(ctx, connStr, false)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create pool: %w", err)
	}

	cleanup := func() {
		pool.Close()
		if err := close.Close(); err != nil {
			log.Fatalf("failed to close pool: %s", err)
		}
		if err := container.Restore(ctx, postgres.WithSnapshotName("migrated")); err != nil {
			log.Fatalf("failed to restore: %s", err)
		}
	}

	return pool, cleanup, nil
}

type pubsubRunner struct {
	*runner.PubSub
}

func newPubsubRunner() *pubsubRunner {
	ret := &pubsubRunner{}
	ret.PubSub = runner.NewPubSub(nil)
	return ret
}

func (p *pubsubRunner) Publish(ctx context.Context, msg protoreflect.ProtoMessage, attrs map[string]string) (string, error) {
	b, err := protojson.Marshal(msg)
	if err != nil {
		return "", err
	}

	mp := make(map[string]any)
	if err := json.Unmarshal(b, &mp); err != nil {
		return "", err
	}

	p.Receive("topic", runner.PubSubMessage{
		Msg:        mp,
		Attributes: attrs,
	})

	return "123", nil
}

func (p *pubsubRunner) String() string {
	return "topic"
}

type naisdRunner struct {
	*runner.PubSub
	topics               map[string]chan pubsubMockMsg
	reconcilerPublishers map[string]workers.Publisher

	statusCh chan pubsubMockMsg
}

type statusReceiver struct {
	*naisdRunner
}

func (s *statusReceiver) Receive(ctx context.Context, f func(ctx context.Context, msg message.Status) error) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-s.statusCh:
			var status message.Status
			if err := json.Unmarshal(msg.msg, &status); err != nil {
				return err
			}

			if err := f(ctx, status); err != nil {
				return err
			}
		}
	}
}

func newNaisd(ctx context.Context, db database.Repo) (*naisdRunner, func(), error) {
	naisdRunner := &naisdRunner{
		statusCh: make(chan pubsubMockMsg),
	}
	naisdRunner.PubSub = runner.NewPubSub(naisdRunner.doPublish)
	naisdRunner.registerTopic("status", naisdRunner.statusCh)

	return naisdRunner, func() {}, nil
}

func (n *naisdRunner) start(ctx context.Context, db database.Repo) error {
	// for _, t := range config.Tenants {
	// 	for _, env := range t.Envs {
	// 		if err := n.configureEnv(ctx, config, db, t, env); err != nil {
	// 			return err
	// 		}
	// 	}
	// }

	log := logrus.New()
	if testing.Verbose() {
		log.Out = os.Stdout
		log.Level = logrus.DebugLevel
	} else {
		log.Out = io.Discard
	}

	rec := workers.NewReceiver(&statusReceiver{naisdRunner: n}, db, log, fake.NewFakeSlackClient(), "test")
	go rec.Run(ctx)
	return nil
}

func (n *naisdRunner) configureEnv(ctx context.Context, db database.Repo, tenant, env string) error {
	ch, mgr, err := newNaisdForEnv(ctx.Done(), tenant, env, n, n.statusCh)
	if err != nil {
		return err
	}
	_ = ch

	go mgr.Run(ctx)

	return nil
}

func newNaisdForEnv(done <-chan struct{}, tenant, env string, naisdRunner *naisdRunner, statusCh chan pubsubMockMsg) (chan pubsubMockMsg, *naisd.DeployManager, error) {
	reconCh := make(chan pubsubMockMsg)
	reconPublisher := &mockPublisher[message.DeployInstruction]{
		topic:    "naisd-" + tenant + "-" + env,
		pubsub:   naisdRunner.PubSub,
		messages: reconCh,
	}
	naisdRunner.registerReconcilerPublisher("naisd-"+tenant+"-"+env, reconPublisher)

	deploySubscriber := &mockSubscriber[message.DeployInstruction]{
		topic:    "naisd-" + tenant + "-" + env,
		messages: reconCh,
		done:     done,
		pubsub:   naisdRunner.PubSub,
	}
	naisdRunner.registerTopic(deploySubscriber.Name(), deploySubscriber.messages)

	statusPublisher := &mockPublisher[message.Status]{
		topic:    "status",
		pubsub:   naisdRunner.PubSub,
		messages: statusCh,
	}

	logr := logrus.New()
	logr.Level = logrus.DebugLevel
	logr.Out = os.Stdout
	logr.Out = io.Discard

	mgr, err := naisd.NewDeployManager(
		deploySubscriber,
		statusPublisher,
		tenant,
		env,
		&naisd.MockExecutor{Logger: logrus.NewEntry(logr), Timeout: 1 * time.Millisecond, NumSuccessful: ptr.To(100)},
		nil,
		&rest.Config{},
		"",
		"",
		logrus.NewEntry(logr),
	)
	return reconCh, mgr, err
}

func (n *naisdRunner) registerReconcilerPublisher(name string, pub workers.Publisher) {
	if n.reconcilerPublishers == nil {
		n.reconcilerPublishers = make(map[string]workers.Publisher)
	}
	n.reconcilerPublishers[name] = pub
}

func (n *naisdRunner) registerTopic(name string, ch chan pubsubMockMsg) {
	if n.topics == nil {
		n.topics = make(map[string]chan pubsubMockMsg)
	}
	n.topics[name] = ch
}

func (n *naisdRunner) doPublish(topic string, msg runner.PubSubMessage) error {
	if ch, ok := n.topics[topic]; ok {
		msgb, err := json.Marshal(msg.Msg)
		if err != nil {
			return err
		}

		ch <- pubsubMockMsg{
			topic: topic,
			msg:   msgb,
		}

		return nil
	}

	return fmt.Errorf("no such topic: %s", topic)
}

type pubsubMockMsg struct {
	topic string
	msg   []byte
}

type mockSubscriber[T any] struct {
	topic    string
	messages chan pubsubMockMsg
	done     <-chan struct{}
	pubsub   *runner.PubSub
}

func (d *mockSubscriber[T]) Name() string {
	return d.topic
}

func (d *mockSubscriber[T]) Synchronous() {}

func (d *mockSubscriber[T]) Receive(ctx context.Context, f func(ctx context.Context, msg T) error) error {
	for {
		select {
		case <-d.done:
			return nil
		case <-ctx.Done():
			return nil
		case msg := <-d.messages:
			var m T
			mp := make(map[string]any)
			if err := json.Unmarshal(msg.msg, &m); err != nil {
				return err
			}
			if err := json.Unmarshal(msg.msg, &mp); err != nil {
				return err
			}
			d.pubsub.Receive(msg.topic, runner.PubSubMessage{Msg: mp})

			if err := f(ctx, m); err != nil {
				return err
			}
		}
	}
}

type mockPublisher[T any] struct {
	topic    string
	messages chan<- pubsubMockMsg
	pubsub   *runner.PubSub
}

func (m *mockPublisher[T]) Publish(ctx context.Context, msg T) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	m.messages <- pubsubMockMsg{
		topic: m.topic,
		msg:   b,
	}

	mp := make(map[string]any)
	if err := json.Unmarshal(b, &mp); err != nil {
		return err
	}
	m.pubsub.Send(m.topic, runner.PubSubMessage{Msg: mp})

	return nil
}

func (m *mockPublisher[T]) Stop() {}
