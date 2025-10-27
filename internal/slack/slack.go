package slack

import (
	"github.com/slack-go/slack"
)

// Slack is a client for sending messages to Slack
type Slack struct {
	client *slack.Client
}

type SlackClient interface {
	PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error)
	PostComment(channelName, messageTS string, msgOptions []slack.MsgOption) error
	UpdateMessage(channelID, timestamp string, msgOptions []slack.MsgOption) (string, string, string, error)
	AddReaction(channelID, timestamp, reaction string) error
	GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption
	GetClusterUpgradeDoneNotificationMessageOptions(tenant, environment string) []slack.MsgOption
	GetClusterUpgradeStuckNotificationMessageOptions(tenant, environment, version, status, lastModified string) []slack.MsgOption
	GetClusterUpgradeProgressMessageOptions(tenant, environment, version, currentPhase, status string, startTime string, mentions string) []slack.MsgOption
	GetFeatureDeployFailedMessageOptions(feature, tenant, environment string) []slack.MsgOption
}

// New creates a new Slack client
func New(token string) SlackClient {
	return &Slack{
		client: slack.New(token),
	}
}

// PostMessage sends a message to a Slack channel
func (s *Slack) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	channelID, timestamp, err := s.client.PostMessage(channelName, msgOptions...)
	if err != nil {
		return "", "", err
	}
	return channelID, timestamp, nil
}

func (s *Slack) PostComment(channelName, messageTS string, msgOptions []slack.MsgOption) error {
	msgOptions = append(msgOptions, slack.MsgOptionTS(messageTS))
	_, _, err := s.client.PostMessage(channelName, msgOptions...)
	return err
}

func (s *Slack) UpdateMessage(channelID, timestamp string, msgOptions []slack.MsgOption) (string, string, string, error) {
	return s.client.UpdateMessage(channelID, timestamp, msgOptions...)
}

func (s *Slack) AddReaction(channelID, timestamp, reaction string) error {
	return s.client.AddReaction(reaction, slack.ItemRef{Channel: channelID, Timestamp: timestamp})
}
