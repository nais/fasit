package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Publisher sends deploy instructions to naisd agents.
type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

type NewPublisher func(topicID string, log *slog.Logger) Publisher

// pubSubDispatcher writes deploy decisions to the database and publishes
// deploy instructions to naisd via Pub/Sub.
type pubSubDispatcher struct {
	querier      reconcilersql.Querier
	newPublisher NewPublisher
	log          *slog.Logger

	publishersMu sync.Mutex
	publishers   map[string]Publisher

	deployMessages metric.Int64Counter
}

func NewPubSubDispatcher(pool *pgxpool.Pool, publisher NewPublisher, meter metric.Meter, log *slog.Logger) (Dispatcher, error) {
	deployMessages, err := meter.Int64Counter("reconciler_deploy_messages", metric.WithDescription("Deploy messages sent by reconciler"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}
	return &pubSubDispatcher{
		querier:        reconcilersql.New(pool),
		newPublisher:   publisher,
		log:            log,
		publishers:     make(map[string]Publisher),
		deployMessages: deployMessages,
	}, nil
}

func (w *pubSubDispatcher) Dispatch(ctx context.Context, decisions []DeployDecision) error {
	var instructions []reconcilersql.CreateDeployInstructionParams
	var statuses []reconcilersql.UpsertReconcileStatusParams

	type publishItem struct {
		topicID     string
		instruction message.DeployInstruction
		envName     string
		tenantName  string
		featureName string
	}
	var toPublish []publishItem

	for _, res := range decisions {
		switch res.Action {
		case ActionDeploy:
			id := uuid.New()
			vals, err := json.Marshal(res.Values)
			if err != nil {
				return fmt.Errorf("marshal values for %s: %w", res.Feature.Name, err)
			}

			instructions = append(instructions, reconcilersql.CreateDeployInstructionParams{
				ID:                  id,
				EnvironmentID:       res.EnvironmentID,
				FeatureName:         res.Feature.Name,
				FeatureVersion:      res.Feature.Version,
				Hash:                res.Hash,
				Vals:                vals,
				FeatureAssignmentID: &res.FeatureAssignmentID,
			})

			statuses = append(statuses, reconcilersql.UpsertReconcileStatusParams{
				FeatureAssignmentID: res.FeatureAssignmentID,
				EnvironmentID:       res.EnvironmentID,
				Status:              model.RolloutStatusCreated.String(),
				Message:             res.Message,
			})

			toPublish = append(toPublish, publishItem{
				topicID: naisdTopicID(res.TenantName, res.EnvironmentName),
				instruction: message.DeployInstruction{
					ID:         id,
					Name:       res.Feature.Name,
					Version:    res.Feature.Version,
					Chart:      res.Feature.Chart,
					ConfigHash: res.Hash,
					Timeout:    res.Feature.Timeout,
					Values:     res.Values,
				},
				envName:     res.EnvironmentName,
				tenantName:  res.TenantName,
				featureName: res.Feature.Name,
			})

		case ActionSkipInProgress, ActionSkipUnchanged:
			statuses = append(statuses, reconcilersql.UpsertReconcileStatusParams{
				FeatureAssignmentID: res.FeatureAssignmentID,
				EnvironmentID:       res.EnvironmentID,
				Status:              res.Status,
				Message:             res.Message,
			})

		case ActionFailMissingDeps, ActionFailMissingConfig, ActionFailRender:
			statuses = append(statuses, reconcilersql.UpsertReconcileStatusParams{
				FeatureAssignmentID: res.FeatureAssignmentID,
				EnvironmentID:       res.EnvironmentID,
				Status:              model.RolloutStatusFailed.String(),
				Message:             res.Message,
			})

		case ActionSkipUnhealthy:
			statuses = append(statuses, reconcilersql.UpsertReconcileStatusParams{
				FeatureAssignmentID: res.FeatureAssignmentID,
				EnvironmentID:       res.EnvironmentID,
				Status:              model.RolloutStatusPending.String(),
				Message:             res.Message,
			})

		case ActionSkipDisabled:
			// No status update for disabled features.
		}
	}

	if len(instructions) > 0 {
		br := w.querier.CreateDeployInstruction(ctx, instructions)
		var batchErr error
		br.Exec(func(i int, err error) {
			if err != nil {
				batchErr = fmt.Errorf("create instruction %d (%s): %w", i, instructions[i].FeatureName, err)
			}
		})
		if batchErr != nil {
			return batchErr
		}
	}

	if len(statuses) > 0 {
		br := w.querier.UpsertReconcileStatus(ctx, statuses)
		var batchErr error
		br.Exec(func(i int, err error) {
			if err != nil {
				batchErr = fmt.Errorf("upsert status %d: %w", i, err)
			}
		})
		if batchErr != nil {
			return batchErr
		}
	}

	for _, item := range toPublish {
		pub := w.publisher(item.topicID)
		if err := pub.Publish(ctx, item.instruction); err != nil {
			w.log.With("err", err, "feature", item.featureName, "env", item.envName).Error("publish deploy instruction")
			continue
		}
		if err := w.querier.SetDeployInstructionStatus(ctx, reconcilersql.SetDeployInstructionStatusParams{
			ID:     item.instruction.ID,
			Status: model.RolloutStatusPending.String(),
		}); err != nil {
			w.log.With("err", err, "id", item.instruction.ID).Error("set instruction status to sent")
		}
		w.deployMessages.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("tenant", item.tenantName),
			attribute.String("environment", item.envName),
			attribute.String("feature", item.featureName),
		)))
	}

	return nil
}

func (w *pubSubDispatcher) publisher(topicID string) Publisher {
	w.publishersMu.Lock()
	defer w.publishersMu.Unlock()
	if p, ok := w.publishers[topicID]; ok {
		return p
	}
	p := w.newPublisher(topicID, w.log)
	w.publishers[topicID] = p
	return p
}

func naisdTopicID(tenantName, envName string) string {
	return "naisd-" + tenantName + "-" + envName
}
