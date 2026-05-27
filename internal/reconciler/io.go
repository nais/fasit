package reconciler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/reconciler/reconcilersql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func (r *Reconciler) writeResults(ctx context.Context, results []renderResult) error {
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
		case actionDeploy:
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

		case actionSkipInProgress:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, res.Status)
			statusMessages = append(statusMessages, res.Message)

		case actionSkipUnchanged:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, res.Status)
			statusMessages = append(statusMessages, res.Message)

		case actionFailMissingDeps, actionFailMissingConfig, actionFailRender:
			statusDepIDs = append(statusDepIDs, res.DeploymentID)
			statusEnvIDs = append(statusEnvIDs, res.EnvironmentID)
			statuses = append(statuses, model.RolloutStatusFailed.String())
			statusMessages = append(statusMessages, res.Message)

		case actionSkipDisabled:
			// No status update for disabled features.
		}
	}

	if len(instrIDs) > 0 {
		if err := r.querier.BulkCreateDeployInstructions(ctx, reconcilersql.BulkCreateDeployInstructionsParams{
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
		if err := r.querier.BulkUpsertDeploymentStatuses(ctx, reconcilersql.BulkUpsertDeploymentStatusesParams{
			DeploymentIds:  statusDepIDs,
			EnvironmentIds: statusEnvIDs,
			Statuses:       statuses,
			Messages:       statusMessages,
		}); err != nil {
			return fmt.Errorf("bulk upsert statuses: %w", err)
		}
	}

	for _, item := range toPublish {
		pub := r.publisher(item.topicID)
		if err := pub.Publish(ctx, item.instruction); err != nil {
			r.log.WithError(err).WithField("feature", item.featureName).WithField("env", item.envName).Error("publish deploy instruction")
			continue
		}
		r.deployMessages.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("tenant", item.tenantName),
			attribute.String("environment", item.envName),
			attribute.String("feature", item.featureName),
		)))
	}

	return nil
}

func naisdTopicID(tenantName, envName string) string {
	return "naisd-" + tenantName + "-" + envName
}
