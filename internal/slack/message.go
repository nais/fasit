package slack

import (
	"fmt"

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

func (s *Slack) GetClusterUpgradeProgressMessageOptions(tenant, environment, version, currentPhase, status string, startTime string, mentions string) []slack.MsgOption {
	// Use a simple, basic structure that should always work
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*:kubernetes: K8s auto-upgrade*\n\n*Tenant:* %s\n*Environment:* %s\n*Version:* %s\n*Started:* %s", tenant, environment, version, startTime), false, false),
			nil, nil,
		),
	}

	// Build progress indicators
	var progressText string
	switch currentPhase {
	case "master":
		switch status {
		case "in_progress":
			progressText = ":hourglass_flowing_sand: Control plane upgrade in progress..."
		case "completed":
			progressText = ":white_check_mark: Control plane upgrade completed\n:hourglass_flowing_sand: Starting node pools upgrade..."
		default:
			progressText = ":hourglass_flowing_sand: Starting control plane upgrade..."
		}
	case "nodepool":
		switch status {
		case "in_progress":
			progressText = ":white_check_mark: Control plane upgrade completed\n:hourglass_flowing_sand: Node pools upgrade in progress..."
		case "completed":
			progressText = ":white_check_mark: Control plane upgrade completed\n:white_check_mark: Node pools upgrade completed\n:tada: Upgrade completed successfully!"
		default:
			progressText = ":white_check_mark: Control plane upgrade completed\n:hourglass_flowing_sand: Preparing node pools upgrade..."
		}
	case "completed":
		progressText = ":white_check_mark: Control plane upgrade completed\n:white_check_mark: Node pools upgrade completed\n:tada: Upgrade completed successfully!"
	case "failed":
		progressText = ":x: Upgrade failed"
	case "stuck":
		progressText = ":warning: Upgrade stuck (>24h) - marked as failed"
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

func (s *Slack) GetClusterUpgradeStuckNotificationMessageOptions(tenant, environment, version, status, lastModified string) []slack.MsgOption {
	blocks := []slack.Block{
		slack.NewSectionBlock(
			slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*:warning: K8s upgrade stuck*\n\n🚨 Cluster upgrade has been stuck for more than 24 hours and was marked as FAILED.\n\n*Tenant:* %s\n*Environment:* %s\n*Target version:* %s\n*Status:* %s\n*Last modified:* %s\n\n<https://fasit.nais.io/clusters#%s|View in Fasit>", tenant, environment, version, status, lastModified, tenant), false, false),
			nil, nil,
		),
	}

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
