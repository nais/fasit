package slack

import (
	"fmt"

	"github.com/slack-go/slack"
)

func (s *Slack) GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("mrkdwn", ":kubernetes: K8s auto-upgrade", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tenant:* %s\n*Environment:* %s\n*Component:* %s\n*Version:* %s\n*Progress*: <https://fasit.nais.io/clusters#%s|Fasit>", tenant, environment, clusterComponent, version, tenant), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	var mentionsBlock *slack.SectionBlock

	if mentions != "" {
		mentionsBlock = slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("\n%s", mentions), false, false), nil, nil)
		blocks = append(blocks, mentionsBlock)
	}

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetClusterUpgradeDoneNotificationMessageOptions(tenant, environment string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("mrkdwn", ":kubernetes: K8s auto-upgrade completed", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Upgrade completed! :tada:\n\n*Tenant:* %s\n*Cluster:* %s", tenant, environment), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("mrkdwn", ":warning: Feature deploy failed", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Feature:* %s\n*Tenant:* %s\n*Environment:* %s", feature, tenant, environment), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetClusterUpgradeProgressMessageOptions(tenant, environment, version, currentPhase, status string, startTime string, mentions string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("mrkdwn", ":kubernetes: K8s auto-upgrade", false, false))

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

	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tenant:* %s\n*Environment:* %s\n*Version:* %s\n*Started:* %s\n\n%s\n\n*Progress*: <https://fasit.nais.io/clusters#%s|Fasit>", tenant, environment, version, startTime, progressText, tenant), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	if mentions != "" {
		mentionsBlock := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("\n%s", mentions), false, false), nil, nil)
		blocks = append(blocks, mentionsBlock)
	}

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetClusterUpgradeStuckNotificationMessageOptions(tenant, environment, version, status, lastModified string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("mrkdwn", ":warning: K8s upgrade stuck", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🚨 Cluster upgrade has been stuck for more than 24 hours and was marked as FAILED.\n\n*Tenant:* %s\n*Environment:* %s\n*Target version:* %s\n*Status:* %s\n*Last modified:* %s\n\n<https://fasit.nais.io/clusters#%s|View in Fasit>", tenant, environment, version, status, lastModified, tenant), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
