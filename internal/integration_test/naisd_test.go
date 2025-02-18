package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/nais/fasit/internal/database"
	integration "github.com/nais/fasit/internal/integration_test"
	"github.com/nais/fasit/internal/integration_test/testmanager/runner"
	"github.com/nais/fasit/internal/message"
	"github.com/nais/fasit/internal/naisd"
	"github.com/nais/fasit/internal/slack/fake"
	"github.com/nais/fasit/internal/workers"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type naisdRunner struct {
	*runner.PubSub
	topics               map[string]chan pubsubMockMsg
	reconcilerPublishers map[string]workers.Publisher

	statusCh chan pubsubMockMsg
}

func newNaisd(ctx context.Context, config *integration.Config, db database.Repo) (*naisdRunner, func(), error) {
	naisdRunner := &naisdRunner{
		statusCh: make(chan pubsubMockMsg),
	}
	naisdRunner.PubSub = runner.NewPubSub(naisdRunner.doPublish)
	naisdRunner.registerTopic("status", naisdRunner.statusCh)

	return naisdRunner, func() {}, nil
}

func (n *naisdRunner) start(ctx context.Context, config *integration.Config, db database.Repo) error {
	for _, t := range config.Tenants {
		for _, env := range t.Envs {
			if err := n.configureEnv(ctx, config, db, t, env); err != nil {
				return err
			}
		}
	}

	log := logrus.New()
	if testing.Verbose() {
		log.Out = os.Stdout
		log.Level = logrus.DebugLevel
	} else {
		log.Out = io.Discard
	}

	statusMgr := &mockSubscriber[message.Status]{
		topic:    "status",
		messages: n.statusCh,
		done:     ctx.Done(),
		pubsub:   n.PubSub,
	}
	rec := workers.NewReceiver(statusMgr, db, logrus.NewEntry(log), fake.NewFakeSlackClient(), "test")
	go rec.Run(ctx)
	return nil
}

func (n *naisdRunner) configureEnv(ctx context.Context, config *integration.Config, db database.Repo, tenant integration.Tenant, env integration.Env) error {
	ch, mgr, err := newNaisdForEnv(ctx.Done(), config, tenant.Name, env, n, n.statusCh)
	if err != nil {
		return err
	}

	if env.NAISD.Enabled {
		go mgr.Run(ctx)
	} else {
		go func() {
			for range ch {
				// drain channel
			}
		}()
	}

	return nil
}

func newNaisdForEnv(done <-chan struct{}, config *integration.Config, tenant string, env integration.Env, naisdRunner *naisdRunner, statusCh chan pubsubMockMsg) (chan pubsubMockMsg, *naisd.DeployManager, error) {
	reconCh := make(chan pubsubMockMsg)
	reconPublisher := &mockPublisher[message.DeployInstruction]{
		topic:    "naisd-" + tenant + "-" + env.Name,
		pubsub:   naisdRunner.PubSub,
		messages: reconCh,
	}
	naisdRunner.registerReconcilerPublisher("naisd-"+tenant+"-"+env.Name, reconPublisher)

	deploySubscriber := &mockSubscriber[message.DeployInstruction]{
		topic:    "naisd-" + tenant + "-" + env.Name,
		messages: reconCh,
		done:     done,
		pubsub:   naisdRunner.PubSub,
	}
	naisdRunner.registerTopic(deploySubscriber.Name(), deploySubscriber.messages)

	statusPublisher := &mockPublisher[message.Status]{
		topic:    "status",
		pubsub:   naisdRunner.PubSub,
		messages: statusCh,
	}

	logr := logrus.New()
	logr.Level = logrus.DebugLevel
	logr.Out = os.Stdout
	logr.Out = io.Discard

	mgr, err := naisd.NewDeployManager(
		deploySubscriber,
		statusPublisher,
		tenant,
		env.Name,
		&naisd.MockExecutor{Logger: logrus.NewEntry(logr), Timeout: 1 * time.Millisecond, NumSuccessful: &env.NAISD.SuccessfullMessages},
		nil,
		&rest.Config{},
		"",
		"",
		logrus.NewEntry(logr),
	)
	return reconCh, mgr, err
}

func (n *naisdRunner) registerReconcilerPublisher(name string, pub workers.Publisher) {
	if n.reconcilerPublishers == nil {
		n.reconcilerPublishers = make(map[string]workers.Publisher)
	}
	n.reconcilerPublishers[name] = pub
}

func (n *naisdRunner) registerTopic(name string, ch chan pubsubMockMsg) {
	if n.topics == nil {
		n.topics = make(map[string]chan pubsubMockMsg)
	}
	n.topics[name] = ch
}

func (n *naisdRunner) doPublish(topic string, msg runner.PubSubMessage) error {
	b, err := json.Marshal(msg.Msg)
	if err != nil {
		return err
	}

	if ch, ok := n.topics[topic]; ok {
		ch <- pubsubMockMsg{
			topic: topic,
			msg:   b,
		}

		return nil
	}

	return fmt.Errorf("no such topic: %s", topic)
}
