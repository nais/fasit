package slack

import (
	"fmt"

	"github.com/nais/fasit/internal/graph/model"
	"github.com/slack-go/slack"
)

func (s *Slack) GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*:x: Feature deploy failed*\n\n*Feature:* %s\n*Tenant:* %s\n*Environment:* %s", feature, tenant, environment), false, false),
			nil, nil,
		),
	}

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetClusterUpgradeProgressMessageOptions(tenant, environment, version string, upgradeStatus model.UpgradeStatus, startTime, mentions string) []slack.MsgOption {
	// Use a simple, basic structure that should always work
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*:kubernetes: K8s auto-upgrade*\n\n*Tenant:* %s\n*Environment:* %s\n*Version:* %s\n*Started:* %s", tenant, environment, version, startTime), false, false),
			nil, nil,
		),
	}

	// Build progress indicators based on UpgradeStatus
	var progressText string
	switch upgradeStatus {
	case model.UpgradeStatusCreated:
		progressText = ":rocket: Starting control plane upgrade..."
	case model.UpgradeStatusWaiting:
		progressText = ":double_vertical_bar: Upgrade is waiting for the configured delay period before starting..."
	case model.UpgradeStatusControlPlaneUpgrade:
		progressText = ":hourglass_flowing_sand: Control plane upgrade in progress..."
	case model.UpgradeStatusNodeUpgrade:
		progressText = ":white_check_mark: Control plane upgrade completed\n:hourglass_flowing_sand: Node pools upgrade in progress..."
	case model.UpgradeStatusDone:
		progressText = ":white_check_mark: Control plane upgrade completed\n:white_check_mark: Node pools upgrade completed\n:tada: Upgrade completed successfully!"
	case model.UpgradeStatusFailed:
		progressText = ":x: Upgrade failed"
	default:
		progressText = ":hourglass_flowing_sand: Upgrade starting..."
	}

	// Add progress section
	progressBlock := slack.NewSectionBlock(
		slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("%s\n\n*Progress*: <https://fasit.nais.io/clusters#%s|Fasit>", progressText, tenant), false, false),
		nil, nil,
	)
	blocks = append(blocks, progressBlock)

	// Add mentions if provided
	if mentions != "" {
		mentionsBlock := slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", mentions, false, false),
			nil, nil,
		)
		blocks = append(blocks, mentionsBlock)
	}

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
