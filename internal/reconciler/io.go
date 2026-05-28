package reconciler

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Publisher sends deploy instructions to naisd agents.
type Publisher interface {
	Publish(ctx context.Context, msg message.DeployInstruction) error
	Stop()
}

// NewPublisher creates a Publisher for a given topic.
type NewPublisher func(topicID string, log logrus.FieldLogger) Publisher

// DBResultWriter writes reconcile results to the database and publishes
// deploy instructions to naisd via Pub/Sub. This is the default writer
// used in production.
type DBResultWriter struct {
	querier      reconcilersql.Querier
	newPublisher NewPublisher
	log          logrus.FieldLogger

	publishersMu sync.Mutex
	publishers   map[string]Publisher

	deployMessages metric.Int64Counter
}

func NewDBResultWriter(querier reconcilersql.Querier, publisher NewPublisher, meter metric.Meter, log logrus.FieldLogger) (*DBResultWriter, error) {
	deployMessages, err := meter.Int64Counter("reconciler_deploy_messages", metric.WithDescription("Deploy messages sent by reconciler"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}
	return &DBResultWriter{
		querier:        querier,
		newPublisher:   publisher,
		log:            log,
		publishers:     make(map[string]Publisher),
		deployMessages: deployMessages,
	}, nil
}

func (w *DBResultWriter) WriteResults(ctx context.Context, results []Result) error {
	var (
		instrIDs       []uuid.UUID
		instrEnvIDs    []uuid.UUID
		instrFeatures  []string
		instrVersions  []string
		instrHashes    []string
		instrValues    [][]byte
		instrDepIDs    []uuid.UUID
		statusDepIDs   []uuid.UUID
		statusEnvIDs   []uuid.UUID
		statuses       []string
		statusMessages []string
	)

	type publishItem struct {
		topicID     string
		instruction message.DeployInstruction
		envName     string
		tenantName  string
		featureName string
	}
	var toPublish []publishItem

	for _, res := range results {
		switch res.Action {
		case ActionDeploy:
			id := uuid.New()
			vals, err := json.Marshal(res.Values)
			if err != nil {
				return fmt.Errorf("marshal values for %s: %w", res.Feature.Name, err)
			}

			instrIDs = append(instrIDs, id)
			instrEnvIDs = append(instrEnvIDs, res.EnvironmentID)
			instrFeatures = append(instrFeatures, res.Feature.Name)
			instrVersions = append(instrVersions, res.Feature.Version)
			instrHashes = append(instrHashes, res.Hash)
			instrValues = append(instrValues, vals)
			instrDepIDs = append(instrDepIDs, res.DeploymentID)

			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, model.RolloutStatusCreated.String())
			statusMessages = append(statusMessages, res.Message)

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

		case ActionSkipInProgress:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, res.Status)
			statusMessages = append(statusMessages, res.Message)

		case ActionSkipUnchanged:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, res.Status)
			statusMessages = append(statusMessages, res.Message)

		case ActionFailMissingDeps, ActionFailMissingConfig, ActionFailRender:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, model.RolloutStatusFailed.String())
			statusMessages = append(statusMessages, res.Message)

		case ActionSkipUnhealthy:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, model.RolloutStatusPending.String())
			statusMessages = append(statusMessages, res.Message)

		case ActionSkipDisabled:
			// No status update for disabled features.
		}
	}

	if len(instrIDs) > 0 {
		if err := w.querier.BulkCreateDeployInstructions(ctx, reconcilersql.BulkCreateDeployInstructionsParams{
			Ids:             instrIDs,
			EnvironmentIds:  instrEnvIDs,
			FeatureNames:    instrFeatures,
			FeatureVersions: instrVersions,
			Hashes:          instrHashes,
			Vals:            instrValues,
			DeploymentIds:   instrDepIDs,
		}); err != nil {
			return fmt.Errorf("bulk create instructions: %w", err)
		}
	}

	if len(statusDepIDs) > 0 {
		if err := w.querier.BulkUpsertDeploymentStatuses(ctx, reconcilersql.BulkUpsertDeploymentStatusesParams{
			DeploymentIds:  statusDepIDs,
			EnvironmentIds: statusEnvIDs,
			Statuses:       statuses,
			Messages:       statusMessages,
		}); err != nil {
			return fmt.Errorf("bulk upsert statuses: %w", err)
		}
	}

	for _, item := range toPublish {
		pub := w.publisher(item.topicID)
		if err := pub.Publish(ctx, item.instruction); err != nil {
			w.log.WithError(err).WithField("feature", item.featureName).WithField("env", item.envName).Error("publish deploy instruction")
			continue
		}
		w.deployMessages.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("tenant", item.tenantName),
			attribute.String("environment", item.envName),
			attribute.String("feature", item.featureName),
		)))
	}

	return nil
}

func (w *DBResultWriter) publisher(topicID string) Publisher {
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
