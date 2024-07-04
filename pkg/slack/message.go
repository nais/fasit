package slack

import (
	"fmt"

	slackapi "github.com/slack-go/slack"
)

func (s *Slack) GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slackapi.MsgOption {
	blocks := []slackapi.Block{}
	headerBlock := slackapi.NewHeaderBlock(slackapi.NewTextBlockObject("plain_text", ":kubernetes: K8s auto-upgrade", false, false))
	text := slackapi.NewSectionBlock(slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("*Tenant:* %s\n*Environment:* %s\n*Component:* %s\n*Version:* %s\n*Progress*: <https://fasit.nais.io/clusters#%s|Fasit>", tenant, environment, clusterComponent, version, tenant), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	var mentionsBlock *slackapi.SectionBlock

	if mentions != "" {
		mentionsBlock = slackapi.NewSectionBlock(slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("\n%s", mentions), false, false), nil, nil)
		blocks = append(blocks, mentionsBlock)
	}

	return []slackapi.MsgOption{
		slackapi.MsgOptionBlocks(blocks...),
	}
}

func (s *Slack) GetClusterUpgradeDoneNotificationMessageOptions(tenant, environment string) []slackapi.MsgOption {
	blocks := []slackapi.Block{}
	headerBlock := slackapi.NewHeaderBlock(slackapi.NewTextBlockObject("plain_text", ":kubernetes: K8s auto-upgrade completed", false, false))
	text := slackapi.NewSectionBlock(slackapi.NewTextBlockObject("mrkdwn", fmt.Sprintf("Upgrade completed! :tada:\n\n*Tenant:* %s\n*Cluster:* %s", tenant, environment), false, false), nil, nil)
	blocks = append(blocks, headerBlock, text)

	return []slackapi.MsgOption{
		slackapi.MsgOptionBlocks(blocks...),
	}
}
