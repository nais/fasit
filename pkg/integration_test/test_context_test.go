package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/dbtest"
	"github.com/nais/fasit/pkg/feature"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/rollout"
	"github.com/nais/fasit/pkg/workers"
	"github.com/sirupsen/logrus"
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
	FeatureManager  *feature.Manager
	Rollout         *rollout.Rollout
	RolloutWorker   *workers.Rollout
	StatusReceiver  *workers.Receiver
	Reconciler      *workers.Reconciler
	Naisd           naisd
}

func NewTestContext(t *testing.T, features []feature.Feature, envName string, envKind model.EnvironmentKind) (*TestContext, func()) {
	tctx := &TestContext{
		TenantID: uuid.New(),
		EnvID:    uuid.New(),
	}
	ctx := context.Background()
	tctx.FeatureManager = &feature.Manager{
		Features: features,
	}

	dbConnString, close := dbtest.DockerSQLPool()
	db, closers, err := database.NewDB(ctx, dbConnString, false)
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

	for _, f := range features {
		_, err = db.Exec(ctx,
			`INSERT INTO "feature_states" (environment_id, feature, enabled, enabled_at) VALUES ($1, $2, true, TIMESTAMP '2022-09-01 10:10:10')`,
			tctx.EnvID, f.Name,
		)
		if err != nil {
			t.Fatal(err)
		}
	}

	tctx.Rollout, _ = rollout.New(ctx, tctx.FeatureManager, tctx.Repo)
	tctx.Rollout.AllowAll = true

	tctx.RolloutWorker = workers.NewRollout(tctx.Repo, log)

	tctx.Naisd = &MockPublisher{
		ch:          make(chan message.Status, 1),
		tenant:      "tenant1",
		environment: envName,
	}

	tctx.StatusReceiver = workers.NewReceiver(tctx.Naisd, tctx.Repo, tctx.RolloutWorker.Notify, log)

	newPublisher := func(projectID string, topicID string, log *logrus.Entry) workers.Publisher {
		return tctx.Naisd
	}

	tctx.Reconciler = workers.NewReconciler(tctx.Repo, tctx.FeatureManager, newPublisher, "xxx", log)

	return tctx, func() {
		close()
		closers.Close()
	}
}

func (t *TestContext) StartListeners() func() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := t.RolloutWorker.Listen(ctx)
		if err != nil {
			if err.Error() == "conn closed" {
				return
			}
			panic(err)
		}
	}()

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

func (t *TestContext) PostRollout(tt *testing.T, feature string, body map[string]any) uuid.UUID {
	tt.Helper()

	ctx := context.Background()
	w := httptest.NewRecorder()
	b, err := json.Marshal(body)
	if err != nil {
		tt.Fatal(err)
	}
	chiCtx := chi.NewRouteContext()
	chiCtx.URLParams.Add("feature", feature)
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

func (t *TestContext) VerifyRollout(summaryID uuid.UUID, want []*model.Rollout, wantStatus model.RolloutStatus) error {
	var pendingRollout []*model.Rollout
	var err error

	ctx := context.Background()

	waitFor(func() bool {
		pendingRollout, err = t.Repo.RolloutsBySummaryID(ctx, summaryID)
		if len(pendingRollout) == 0 {
			return false
		}

		for _, r := range pendingRollout {
			if r.Status != wantStatus {
				return false
			}
		}
		return true
	})
	if err != nil {
		return fmt.Errorf("repo.RolloutsBySummaryID(ctx, %v) = _, %v, want _, nil", summaryID, err)
	}

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.Rollout{}, "Created", "LastModified", "ID"),
	}

	if !cmp.Equal(want, pendingRollout, cmpOpts...) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(want, pendingRollout, cmpOpts...))
	}
	return nil
}

func (t *TestContext) VerifyEnvConfiguration(featureName string, want []*model.EnvConfiguration) error {
	ctx := context.Background()

	var confs []*model.EnvConfiguration
	var err error

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.EnvConfiguration{}, "ID", "Created"),
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

func (t *TestContext) VerifyGlobalConfiguration(featureName string, want []*model.GlobalConfiguration) error {
	ctx := context.Background()
	var confs []*model.GlobalConfiguration
	var err error
	waitFor(func() bool {
		confs, err = t.Repo.ConfigGet(ctx, featureName)
		return len(confs) > 0
	})
	if err != nil {
		return fmt.Errorf("repo.ConfigGet(ctx, %v) = _, %v, want _, nil", featureName, err)
	}

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreFields(model.GlobalConfiguration{}, "ID", "Created"),
	}

	if !cmp.Equal(want, confs, cmpOpts...) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(want, confs, cmpOpts...))
	}
	return nil
}

func (t *TestContext) VerifyDeployInstructions(want []message.DeployInstruction) error {
	waitFor(func() bool {
		return len(t.Naisd.DeployInstructions()) > 0
	})
	if !cmp.Equal(want, t.Naisd.DeployInstructions()) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(want, t.Naisd.DeployInstructions()))
	}
	return nil
}

func (t *TestContext) RolloutSummaryRolloutIDs(tb testing.TB, ctx context.Context, summaryID uuid.UUID) []uuid.UUID {
	tb.Helper()

	rollouts, err := t.Repo.RolloutsBySummaryID(ctx, summaryID)
	if err != nil {
		tb.Fatalf("repo.RolloutsBySummaryID(ctx, %v) = _, %v, want _, nil", summaryID, err)
	}

	var ids []uuid.UUID
	for _, r := range rollouts {
		ids = append(ids, r.ID)
	}

	return ids
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
