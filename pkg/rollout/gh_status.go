package rollout

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v56/github"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/database/notifier"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/sirupsen/logrus"
)

type GHStatusReporter struct {
	log      logrus.FieldLogger
	notifier *notifier.Notifier
	client   *github.Client
	repo     database.RolloutRepo
}

func NewGHStatusReporter(log logrus.FieldLogger, repo database.RolloutRepo, notif *notifier.Notifier, pemPath string) (*GHStatusReporter, error) {
	const appID = 415736

	itr, err := ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, appID, pemPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %w", err)
	}

	// Use installation transport with client.
	client := github.NewClient(&http.Client{Transport: itr})

	return &GHStatusReporter{
		log:      log,
		notifier: notif,
		client:   client,
		repo:     repo,
	}, nil
}

func (r *GHStatusReporter) Run(ctx context.Context) {
	rollouts := r.notifier.Listen("rollouts")
	for {
		select {
		case <-ctx.Done():
			return
		case roll := <-rollouts:
			r.report(ctx, roll)
		}
	}
}

func (r *GHStatusReporter) report(ctx context.Context, payload notifier.Payload) {
	ida, ok := payload.Data["id"]
	if !ok {
		r.log.Error("missing id in payload")
		return
	}

	ids, ok := ida.(string)
	if !ok {
		r.log.Error("id is not a string")
		return
	}

	id, err := uuid.Parse(ids)
	if err != nil {
		r.log.WithError(err).Error("failed to parse id")
		return
	}

	rollout, err := r.repo.RolloutByID(ctx, id)
	if err != nil {
		r.log.WithField("id", ids).WithError(err).Error("failed to fetch rollout")
		return
	}

	if rollout.GHRef == nil {
		r.log.WithField("id", id).Debug("no gh ref")
		return
	}

	state := ghState(rollout.Status)

	_, _, err = r.client.Repositories.CreateStatus(ctx, rollout.GHRef.Owner, rollout.GHRef.Repo, rollout.GHRef.Ref, &github.RepoStatus{
		TargetURL:   github.String(fmt.Sprintf("https://fasit.nais.io/features/%v/rollouts/%v", rollout.FeatureName, rollout.Version)),
		State:       state,
		Description: github.String("Rollout of feature"),
		Context:     github.String("fasit / " + rollout.FeatureName),
	})
	if err != nil {
		r.log.WithError(err).Error("failed to create status")
		return
	}
}

func ghState(status model.RolloutStatus) *string {
	s := "pending"
	switch status {
	case model.RolloutStatusDeployed:
		s = "success"
	case model.RolloutStatusFailed:
		s = "failure"
	}

	return &s
}
