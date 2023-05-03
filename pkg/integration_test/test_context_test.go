package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/nais/fasit/pkg/workers"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/metric"
)

type naisd interface {
	workers.Publisher
	workers.ReceiverClient
	DeployInstructions() []message.DeployInstruction
	SendStatus(status model.RolloutStatus)
}

type TestContext struct {
	TenantID        uuid.UUID
	EnvID           uuid.UUID
	EnvManagementID uuid.UUID
	Repo            database.Repo
	DB              *pgxpool.Pool
	StatusReceiver  *workers.Receiver
	Reconciler      *workers.Reconciler
	Naisd           naisd
	Rollout         rollout.Rollout
}

func NewTestContext(t *testing.T, envName string, envKind model.EnvironmentKind) (*TestContext, func()) {
	tctx := &TestContext{
		TenantID: uuid.New(),
		EnvID:    uuid.New(),
	}
	ctx := context.Background()

	dbConnString, close := dbtest.DockerSQLPool()
	db, closers, err := database.NewDB(ctx, dbConnString+" pool_max_conns=5", false)
	if err != nil {
		t.Fatal(err)
	}

	tctx.DB = db

	logrus.StandardLogger().Level = logrus.DebugLevel
	log := logrus.NewEntry(logrus.StandardLogger())

	tctx.Repo = database.New(db, log)

	if err := database.Migrate("pgx", dbConnString, log); err != nil {
		t.Fatalf("error migrating database: %v", err)
	}

	_, err = db.Exec(ctx, `INSERT INTO tenants (id, name, ci) VALUES ($1, 'tenant1', true)`, tctx.TenantID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(ctx, `INSERT INTO environments (id, tenant_id, name, kind, ci) VALUES ($1, $2, $3, $4, true)`, tctx.EnvID, tctx.TenantID, envName, envKind)
	if err != nil {
		t.Fatal(err)
	}

	// for _, f := range features {
	// 	_, err = db.Exec(ctx,
	// 		`INSERT INTO "feature_states" (environment_id, feature, enabled, enabled_at) VALUES ($1, $2, true, TIMESTAMP '2022-09-01 10:10:10')`,
	// 		tctx.EnvID, f.Name,
	// 	)
	// 	if err != nil {
	// 		t.Fatal(err)
	// 	}
	// }

	tctx.Naisd = &MockPublisher{
		ch:          make(chan message.Status, 1),
		tenant:      "tenant1",
		environment: envName,
	}

	tctx.StatusReceiver = workers.NewReceiver(tctx.Naisd, tctx.Repo, log)

	newPublisher := func(projectID string, topicID string, log *logrus.Entry) workers.Publisher {
		return tctx.Naisd
	}

	meter := metric.NewNoopMeterProvider().Meter("")
	tctx.Reconciler, err = workers.NewReconciler(tctx.Repo, newPublisher, "xxx", meter, log)
	_ = err

	return tctx, func() {
		close()
		closers.Close()
	}
}

func (t *TestContext) StartListeners() func() {
	ctx, cancel := context.WithCancel(context.Background())

	go t.StatusReceiver.Run(ctx)

	go func() {
		err := t.Reconciler.Listen(ctx)
		if err != nil {
			if err.Error() == "conn closed" {
				return
			}
			panic(err)
		}
	}()

	return cancel
}

func (t *TestContext) VerifyEnvConfiguration(featureName string, want []*model.Configuration) error {
	ctx := context.Background()

	var confs []*model.Configuration
	var err error

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created"),
	}

	waitFor(func() bool {
		confs, err = t.Repo.ConfigGetForEnv(ctx, featureName, t.EnvID)
		return cmp.Equal(want, confs, cmpOpts...)
	})
	if err != nil {
		return fmt.Errorf("repo.ConfigGetForEnv(ctx, %v, %v) = _, %v, want _, nil", featureName, t.EnvID, err)
	}

	if !cmp.Equal(want, confs, cmpOpts...) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(want, confs, cmpOpts...))
	}
	return nil
}

func (t *TestContext) VerifyGlobalConfiguration(featureName string, want []*model.Configuration) error {
	ctx := context.Background()
	var confs []*model.Configuration
	var err error
	waitFor(func() bool {
		confs, err = t.Repo.ConfigGet(ctx, featureName)
		return len(confs) > 0
	})
	if err != nil {
		return fmt.Errorf("repo.ConfigGet(ctx, %v) = _, %v, want _, nil", featureName, err)
	}

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.Configuration{}, "ID", "Created"),
	}

	if !cmp.Equal(want, confs, cmpOpts...) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(want, confs, cmpOpts...))
	}
	return nil
}

func (t *TestContext) VerifyDeployInstructions(want []message.DeployInstruction) error {
	waitFor(func() bool {
		return len(t.Naisd.DeployInstructions()) >= len(want)
	})
	if !cmp.Equal(want, t.Naisd.DeployInstructions()) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(want, t.Naisd.DeployInstructions()))
	}
	return nil
}

func (t *TestContext) DebugQuery(q string) {
	rows, err := t.DB.Query(context.Background(), q)
	if err != nil {
		panic(err)
	}

	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			panic(err)
		}

		for _, v := range vals {
			fmt.Print(v, "\t")
		}
		fmt.Println()
	}
}

func (t *TestContext) PostRollout(tt *testing.T, body rollout.Request) uuid.UUID {
	tt.Helper()

	ctx := context.Background()
	w := httptest.NewRecorder()
	b, err := json.Marshal(body)
	if err != nil {
		tt.Fatal(err)
	}
	chiCtx := chi.NewRouteContext()
	req, err := http.NewRequestWithContext(context.WithValue(ctx, chi.RouteCtxKey, chiCtx), "POST", "/rollout", bytes.NewReader(b))
	if err != nil {
		tt.Fatal(err)
	}

	t.Rollout.Rollout(w, req)

	if w.Code != 201 {
		tt.Log(w.Body.String())
		tt.Fatalf("got %v, want 201", w.Code)
	}

	obj := struct {
		Rollout         uuid.UUID `json:"rollout"`
		EnvNotAvailable []string  `json:"envNotAvailable"`
	}{}

	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		tt.Fatalf("json.Unmarshal(jsonBody, v) = %v, want nil", err)
	}

	if len(obj.EnvNotAvailable) > 0 {
		tt.Fatalf("got %v, feature not enabled in given environments", obj.EnvNotAvailable)
	}

	return obj.Rollout
}

func (t *TestContext) VerifyRollout(rolloutID uuid.UUID, name, version string, wantStatus model.RolloutStatus, featureData featureDataRow) error {
	var err error

	ctx := context.Background()

	rolloutRow := &struct {
		Name, Version string
		Status        model.RolloutStatus
	}{}

	waitFor(func() bool {
		if err := t.DB.QueryRow(ctx, `SELECT * FROM rollouts WHERE id = $1`, rolloutID).Scan(rolloutRow); err != nil {
			return false
		}
		if rolloutRow.Status != wantStatus {
			return false
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("error getting rollout(%v): %w", rolloutID, err)
	}

	fd := &featureDataRow{}
	if err := t.DB.QueryRow(ctx, `SELECT * FROM feature_data WHERE name = $1 AND version = $2`, name, version).Scan(fd); err != nil {
		return fmt.Errorf("error getting feature_data(%v, %v): %w", name, version, err)
	}

	if !cmp.Equal(featureData, fd) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(featureData, fd))
	}
	return nil
}

type featureDataRow struct {
	Chart         string
	Description   string
	Source        string
	Kinds         []model.EnvironmentKind
	Dependencies  pgtype.JSONB
	Values        pgtype.JSONB
	Timeout       *time.Duration
	DefaultValues pgtype.JSONB
}
