package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/nais/fasit"
	"github.com/nais/fasit/pkg/auth"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph"
	"github.com/nais/fasit/pkg/graph/graphgen"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/provider"
	"github.com/nais/fasit/pkg/provider/protogen"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/nais/fasit/pkg/workers"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	// Supported database drivers.
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	_ "github.com/lib/pq"
)

var cfg = DefaultConfig()

func init() {
	flag.StringVar(&cfg.BindAddress, "bind-address", cfg.BindAddress, "Bind address")
	flag.StringVar(&cfg.GRPCBindAddress, "grpc-bind-address", cfg.GRPCBindAddress, "Bind address")
	flag.StringVar(&cfg.DBConnectionDSN, "db-connection-dsn", getEnv("FASIT_DBCONN_STRING", "postgres://postgres:postgres@127.0.0.1:5432/fasit?sslmode=disable"), "database connection DSN")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "which log level to output")
	flag.StringVar(&cfg.GCPProjectID, "project-id", "nais-local-dev", "Google project ID")
	flag.StringVar(&cfg.StatusSubscriptionID, "status-subscription-id", "fasit-subscription", "Pub/sub subscription for status")
	flag.StringVar(&cfg.IAPAudience, "iap-audience", "", "IAP audience string")
	flag.BoolVar(&cfg.InsecureSkipProxy, "insecure-skip-proxy", false, "Insecure, but allows the server to not require iap")
	flag.BoolVar(&cfg.InsecureSkipTokenCheck, "insecure-skip-token-check", false, "Insecure, but allows the server ignore token check")
}

func newServer(es graphql.ExecutableSchema) *handler.Server {
	srv := handler.New(es)

	srv.AddTransport(transport.Websocket{
		KeepAlivePingInterval: 10 * time.Second,
		Upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool {
			return true
		}},
	})
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.AddTransport(transport.MultipartForm{})

	srv.SetQueryCache(lru.New(1000))

	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New(100),
	})

	return srv
}

func main() {
	flag.Parse()

	// ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	ctx := context.Background()
	// defer cancel()

	log := newLogger()

	log.Info("starting pubsub client")
	client, err := pubsub.NewClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.WithError(err).Fatal("setting up pubsub client")
	}
	log.Info("-- successfully started pubsub client")

	log.Info("starting database client")
	dbDriver := "pgx"
	if !strings.Contains(cfg.DBConnectionDSN, "://") {
		dbDriver = "cloudsql-postgres"
	}

	db, closers, err := database.NewDB(ctx, cfg.DBConnectionDSN, dbDriver != "pgx")
	if err != nil {
		log.WithError(err).Fatal("setting up database")
	}
	defer func() {
		if err := closers.Close(); err != nil {
			log.WithError(err).Errorf("closing database: %v", err)
		}
	}()

	if err := database.Migrate(dbDriver, cfg.DBConnectionDSN, log.WithField("subsystem", "migrate")); err != nil {
		log.WithError(err).Fatal("migrating database")
	}

	repo := database.New(db, log.WithField("subsystem", "repo"))
	log.Info("-- successfully started database client")

	featureMgr, err := feature.New(fasit.FeaturesFS)
	if err != nil {
		log.WithError(err).Fatal("setting up features")
	}

	statusMgr := message.NewSubscriber[message.Status](client, cfg.GCPProjectID, cfg.StatusSubscriptionID)

	rolloutWorker := workers.NewRollout(repo, log.WithField("subsystem", "rollout"))
	go func() {
		if err := rolloutWorker.Listen(ctx); err != nil {
			log.WithError(err).Fatal("rollout worker listener")
		}
	}()
	go rolloutWorker.Run(ctx, 10*time.Minute)

	receiver := workers.NewReceiver(statusMgr, repo, rolloutWorker.Notify, log.WithField("subsystem", "status"))
	go receiver.Run(ctx)

	createPublisher := func(projectID, topicID string, log *logrus.Entry) workers.Publisher {
		return message.NewPublisher[message.DeployInstruction](client, projectID, topicID, log)
	}
	reconciler := workers.NewReconciler(repo, featureMgr, createPublisher, cfg.GCPProjectID, log.WithField("subsystem", "reconciler"))
	go func() {
		defer log.Error("reconciler listener stopped")
		if err := reconciler.Listen(ctx); err != nil {
			log.WithError(err).Fatal("setting up reconciler listener")
		}
	}()
	go reconciler.Run(ctx, 10*time.Minute)

	resolver := &graph.Resolver{
		Repo:     repo,
		Features: featureMgr,
		Log:      log.WithField("subsystem", "graphql"),
	}

	srv := newServer(graphgen.NewExecutableSchema(graphgen.Config{Resolvers: resolver}))

	corsMW := cors.New(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowCredentials: true,
	})

	// Add the IAP validation middleware.
	// If the IAP audience is not set, we stop the server with a fatal error
	// unless the insecure-skip-proxy flag is set.
	iapMW := auth.ValidateJWTFromComputeEngine(cfg.IAPAudience)
	if cfg.IAPAudience == "" {
		if !cfg.InsecureSkipProxy {
			log.Fatal("IAP audience must be set")
		}

		iapMW = auth.InsecureValidateMW
	}

	router := chi.NewMux()
	router.Handle("/", iapMW(playground.Handler("GraphQL playground", "/query")))
	router.Handle("/query", iapMW(corsMW.Handler(srv)))

	rout, err := rollout.New(ctx, featureMgr, repo)
	if err != nil {
		log.WithError(err).Fatal("setting up rollout")
	}
	rout.AllowAll = cfg.InsecureSkipTokenCheck
	router.Post("/github/deploy/{feature}", rout.Rollout)

	go func() {
		if err := runGRPC(ctx, repo); err != nil {
			panic(err)
		}
	}()

	log.Printf("connect to http://%s/ for GraphQL playground", cfg.BindAddress)
	log.Fatal(http.ListenAndServe(cfg.BindAddress, router))
}

func newLogger() *logrus.Logger {
	log := logrus.StandardLogger()
	log.SetFormatter(&logrus.JSONFormatter{})

	l, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Fatal(err)
	}
	log.SetLevel(l)
	return log
}

func getEnv(key, fallback string) string {
	if env := os.Getenv(key); env != "" {
		return env
	}
	return fallback
}

func runGRPC(ctx context.Context, repo database.Repo) error {
	fmt.Println("GRPC serving on port", cfg.GRPCBindAddress)
	lis, err := net.Listen("tcp", cfg.GRPCBindAddress)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	opts := []grpc.ServerOption{}
	s := grpc.NewServer(opts...)

	protogen.RegisterProviderServer(s, provider.NewServer(repo))

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return s.Serve(lis) })
	g.Go(func() error {
		<-ctx.Done()
		s.GracefulStop()
		return nil
	})

	return g.Wait()
}
