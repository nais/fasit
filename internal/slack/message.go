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
