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

// pubSubDeployer writes deploy decisions to the database and publishes
// deploy instructions to naisd via Pub/Sub.
type pubSubDeployer struct {
	querier      reconcilersql.Querier
	newPublisher NewPublisher
	log          *slog.Logger

	publishersMu sync.Mutex
	publishers   map[string]Publisher

	deployMessages metric.Int64Counter
}

func NewPubSubDeployer(pool *pgxpool.Pool, publisher NewPublisher, meter metric.Meter, log *slog.Logger) (Deployer, error) {
	deployMessages, err := meter.Int64Counter("reconciler_deploy_messages", metric.WithDescription("Deploy messages sent by reconciler"))
	if err != nil {
		return nil, fmt.Errorf("create deploy messages counter: %w", err)
	}
	return &pubSubDeployer{
		querier:        reconcilersql.New(pool),
		newPublisher:   publisher,
		log:            log,
		publishers:     make(map[string]Publisher),
		deployMessages: deployMessages,
	}, nil
}

func (w *pubSubDeployer) Deploy(ctx context.Context, decisions []ComputeResult) error {
	type publishItem struct {
		topicID     string
		diid        uuid.UUID
		vals        []byte
		instruction message.DeployInstruction
		res         ComputeResult
	}
	var toPublish []publishItem

	for _, res := range decisions {
		if res.Action != ActionDeploy {
			continue
		}
		diid := uuid.New()
		vals, err := json.Marshal(res.Values)
		if err != nil {
			return fmt.Errorf("marshal values for %s: %w", res.Feature.Name, err)
		}
		toPublish = append(toPublish, publishItem{
			topicID: naisdTopicID(res.TenantName, res.EnvironmentName),
			diid:    diid,
			vals:    vals,
			instruction: message.DeployInstruction{
				ID:         diid,
				Name:       res.Feature.Name,
				Version:    res.Feature.Version,
				Chart:      res.Feature.Chart,
				ConfigHash: res.Hash,
				Timeout:    res.Feature.Timeout,
				Values:     res.Values,
			},
			res: res,
		})
	}

	// Append-only: a deploy_log row is written only after a successful publish,
	// carrying the diid sent to naisd, the hash, and the rendered values (status
	// pending). naisd later appends the terminal row for the same diid. A publish
	// failure writes nothing, so the next cycle sees the previous deploy and
	// retries.
	var deployRows []reconcilersql.AppendDeploysParams
	for _, item := range toPublish {
		pub := w.publisher(item.topicID)
		if err := pub.Publish(ctx, item.instruction); err != nil {
			w.log.With("err", err, "feature", item.res.Feature.Name, "env", item.res.EnvironmentName).Error("publish deploy instruction")
			continue
		}

		deployRows = append(deployRows, reconcilersql.AppendDeploysParams{
			Diid:                item.diid,
			EnvironmentID:       item.res.EnvironmentID,
			FeatureAssignmentID: item.res.FeatureAssignmentID,
			FeatureName:         item.res.Feature.Name,
			FeatureVersion:      item.res.Feature.Version,
			Status:              model.RolloutStatusPending.String(),
			Hash:                item.res.Hash,
			Vals:                item.vals,
		})
		w.deployMessages.Add(ctx, 1, metric.WithAttributeSet(attribute.NewSet(
			attribute.String("tenant", item.res.TenantName),
			attribute.String("environment", item.res.EnvironmentName),
			attribute.String("feature", item.res.Feature.Name),
		)))
	}

	if len(deployRows) > 0 {
		if _, err := w.querier.AppendDeploys(ctx, deployRows); err != nil {
			return fmt.Errorf("append deploys: %w", err)
		}
	}

	return nil
}

func (w *pubSubDeployer) publisher(topicID string) Publisher {
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
