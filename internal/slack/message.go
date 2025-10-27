package slack

import (
	"fmt"

	"github.com/slack-go/slack"
)

func (s *Slack) GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", ":kubernetes: K8s auto-upgrade", false, false))
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
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", ":kubernetes: K8s auto-upgrade completed", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("Upgrade completed! :tada:\n\n*Tenant:* %s\n*Cluster:* %s", tenant, environment), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", ":warning: Feature deploy failed", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Feature:* %s\n*Tenant:* %s\n*Environment:* %s", feature, tenant, environment), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetClusterUpgradeStuckNotificationMessageOptions(tenant, environment, version, status, lastModified string) []slack.MsgOption {
	blocks := []slack.Block{}
	headerBlock := slack.NewHeaderBlock(slack.NewTextBlockObject("plain_text", ":warning: K8s upgrade stuck", false, false))
	text := slack.NewSectionBlock(slack.NewTextBlockObject("mrkdwn", fmt.Sprintf("🚨 Cluster upgrade has been stuck for more than 24 hours and was marked as FAILED.\n\n*Tenant:* %s\n*Environment:* %s\n*Target version:* %s\n*Status:* %s\n*Last modified:* %s\n\n<https://fasit.nais.io/clusters#%s|View in Fasit>", tenant, environment, version, status, lastModified, tenant), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slack.MsgOption{
		slack.MsgOptionBlocks(blocks...),
	}
}
