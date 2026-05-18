package workers

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/database/gensql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/provider/protogen"
)

// fakeTxRepo implements database.Repo for transaction callbacks in tests.
// Only release-status methods are wired; everything else panics.
type fakeTxRepo struct {
	deleteFunc func(ctx context.Context, id uuid.UUID) error
	createFunc func(ctx context.Context, envID uuid.UUID, r *message.Release) error
}

func (f *fakeTxRepo) ReleaseStatusDeleteByEnvironmentID(ctx context.Context, id uuid.UUID) error {
	return f.deleteFunc(ctx, id)
}

func (f *fakeTxRepo) ReleaseStatusCreateOrUpdate(ctx context.Context, envID uuid.UUID, r *message.Release) error {
	return f.createFunc(ctx, envID, r)
}

func (f *fakeTxRepo) ReleaseStatusesGet(context.Context, uuid.UUID) ([]*model.Release, error) {
	panic("not called")
}

// --- unimplemented Repo methods ---

func (f *fakeTxRepo) Close()                                                {}
func (f *fakeTxRepo) WithTx(context.Context) (database.Repo, pgx.Tx, error) { panic("not called") }
func (f *fakeTxRepo) TxFunc(context.Context, database.TXFunc) error         { panic("not called") }
func (f *fakeTxRepo) DeployInstructionsForFeature(context.Context, uuid.UUID, string, int) ([]*model.DeployInstruction, error) {
	panic("not called")
}

func (f *fakeTxRepo) DeployInstructionsLatestForFeature(context.Context, uuid.UUID, string) (*model.DeployInstruction, error) {
	panic("not called")
}

func (f *fakeTxRepo) DeployInstructionsLatestDeployedForFeature(context.Context, uuid.UUID, string) (*model.DeployInstruction, error) {
	panic("not called")
}

func (f *fakeTxRepo) DeployInstructionUpdateStatus(context.Context, uuid.UUID, model.RolloutStatus) error {
	panic("not called")
}

func (f *fakeTxRepo) HelmValueDiffGet(context.Context, *model.DeployInstruction, []string) (*model.HelmValueDiff, error) {
	panic("not called")
}

func (f *fakeTxRepo) NamesFromDeployInstruction(context.Context, uuid.UUID) (string, string, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentByNames(context.Context, string, string) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentCI(context.Context, model.EnvironmentKind) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentCreate(context.Context, *model.EnvironmentCreate) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentGet(context.Context, uuid.UUID) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentGetByName(context.Context, uuid.UUID, string) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentIDByNames(context.Context, string, string) (uuid.UUID, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentsGet(context.Context, uuid.UUID) ([]*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentUpdate(context.Context, uuid.UUID, *model.EnvironmentUpdate) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentSetReconcile(context.Context, uuid.UUID, bool) (*model.Environment, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentSetLabels(context.Context, uuid.UUID, []*protogen.EnvironmentLabel) error {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentGetLabels(context.Context, uuid.UUID) ([]*model.EnvironmentLabel, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentValueDelete(context.Context, uuid.UUID, string) error {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentValueGet(context.Context, uuid.UUID, string, bool) (*model.EnvironmentValue, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentValuesForEnvironment(context.Context, uuid.UUID, bool) ([]*model.EnvironmentValue, error) {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentValueStore(context.Context, uuid.UUID, string, json.RawMessage, bool) error {
	panic("not called")
}

func (f *fakeTxRepo) EnvironmentValuesAcrossEnvs(context.Context, string) ([]gensql.EnvironmentValuesAcrossEnvsRow, error) {
	panic("not called")
}

var _ database.Repo = (*fakeTxRepo)(nil)
