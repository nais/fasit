package featureassignment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/database/types"
	"github.com/nais/fasit/internal/environment"
	featurepkg "github.com/nais/fasit/internal/feature"
	"github.com/nais/fasit/internal/featureassignment/featureassignmentsql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	commonmodel "github.com/nais/fasit/internal/model"
	"go.opentelemetry.io/otel/metric"
)

type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log *slog.Logger) Publisher

type deployer struct {
	newPublisher   NewPublisher
	querier        featureassignmentsql.Querier
	pool           *pgxpool.Pool
	log            *slog.Logger
	deployMessages metric.Int64Counter

	publishersMu sync.Mutex
	publishers   map[string]Publisher
}

func newDeployer(pool *pgxpool.Pool, querier featureassignmentsql.Querier, publisher NewPublisher, meter metric.Meter, log *slog.Logger) (*deployer, error) {
	deployMessages, err := meter.Int64Counter("assignment_deploy_messages", metric.WithDescription("Deploy messages sent"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}

	return &deployer{
		newPublisher:   publisher,
		querier:        querier,
		pool:           pool,
		log:            log,
		deployMessages: deployMessages,
		publishers:     make(map[string]Publisher),
	}, nil
}

func (d *deployer) CreateFeatureAssignment(ctx context.Context, feat *model.Feature, description *string, githubRef *commonmodel.GitHubCommit, target environment.Labels) (uuid.UUID, error) {
	details, err := featurepkg.ParseTemplateDetails(feat.Values)
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to parse feature template details: %w", err)
	}

	if err := featurepkg.FeatureDataCreate(ctx, *feat, details); err != nil {
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23505" {
			return uuid.Nil, fmt.Errorf("unable to create feature data: %w", pgErr)
		}
	}

	var ghRef []byte
	if githubRef != nil {
		b, err := json.Marshal(githubRef.Ref)
		if err != nil {
			return uuid.Nil, fmt.Errorf("marshal gh ref: %w", err)
		}

		ghRef = b
	}

	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	txQuerier := featureassignmentsql.New(tx)

	err = txQuerier.DeactivateActiveFeatureAssignmentForTarget(ctx, featureassignmentsql.DeactivateActiveFeatureAssignmentForTargetParams{
		FeatureName: feat.Name,
		Target:      types.EnvironmentLabels(target),
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("deactivate previous assignment: %w", err)
	}

	assignment, err := txQuerier.CreateFeatureAssignment(ctx, featureassignmentsql.CreateFeatureAssignmentParams{
		FeatureName: feat.Name,
		Version:     feat.Version,
		GhRef:       ghRef,
		Target:      types.EnvironmentLabels(target),
		Description: description,
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to create feature assignment: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit tx: %w", err)
	}

	return assignment.ID, nil
}

// IsMoreSpecific reports whether a deployment with candidateLabels (created at
// candidateCreated) should replace one with existingLabels (created at
// existingCreated). More target labels means more specific. Equal count: latest wins.
func IsMoreSpecific(candidateLabels, existingLabels map[string]string, candidateCreated, existingCreated time.Time) bool {
	if len(candidateLabels) > len(existingLabels) {
		return true
	}
	return len(candidateLabels) == len(existingLabels) && candidateCreated.After(existingCreated)
}

// mostSpecificPerFeature picks one feature assignment per feature name: the one with
// the most specific target labels (most labels wins), breaking ties by latest
// created timestamp.
func mostSpecificPerFeature(deps []*FeatureAssignment) []*FeatureAssignment {
	assignments := map[string]*FeatureAssignment{}
	for _, dep := range deps {
		existing, ok := assignments[dep.Feature.Name]
		if !ok || IsMoreSpecific(dep.TargetLabels, existing.TargetLabels, dep.Created, existing.Created) {
			assignments[dep.Feature.Name] = dep
		}
	}

	ret := make([]*FeatureAssignment, 0)
	for _, d := range assignments {
		ret = append(ret, d)
	}

	slices.SortStableFunc(ret, func(a, b *FeatureAssignment) int {
		return a.Created.Compare(b.Created)
	})

	return ret
}
