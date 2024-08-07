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
	PostComment(channelName, messageTs string, msgOptions []slack.MsgOption) error
	AddReaction(channelId, timestamp, reaction string) error
	GetClusterUpgradeNotificationMessageOptions(tenant, environment, version, clusterComponent, mentions string) []slack.MsgOption
	GetClusterUpgradeDoneNotificationMessageOptions(tenant, environment string) []slack.MsgOption
	GetFeatureDeployFailed(feature, tenant, environment string) []slack.MsgOption
}

// New creates a new Slack client
func New(token string) SlackClient {
	return &Slack{
		client: slack.New(token),
	}
}

// SendMessage sends a message to a Slack channel
func (s *Slack) PostMessage(channelName string, msgOptions []slack.MsgOption) (string, string, error) {
	channelId, timestamp, err := s.client.PostMessage(channelName, msgOptions...)
	if err != nil {
		return "", "", err
	}
	return channelId, timestamp, nil
}

func (s *Slack) PostComment(channelName, messageTs string, msgOptions []slack.MsgOption) error {
	msgOptions = append(msgOptions, slack.MsgOptionTS(messageTs))
	_, _, err := s.client.PostMessage(channelName, msgOptions...)
	return err
}

func (s *Slack) AddReaction(channelId, timestamp, reaction string) error {
	return s.client.AddReaction(reaction, slack.ItemRef{Channel: channelId, Timestamp: timestamp})
}
