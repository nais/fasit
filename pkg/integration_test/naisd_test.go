package integration_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nais/fasit/pkg/database"
	"github.com/nais/fasit/pkg/integration_test/testmanager"
	"github.com/nais/fasit/pkg/integration_test/testmanager/runner"
	"github.com/nais/fasit/pkg/message"
	"github.com/nais/fasit/pkg/naisd"
	"github.com/nais/fasit/pkg/workers"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type naisdRunner struct {
	*runner.PubSub
	topics               map[string]chan pubsubMockMsg
	reconcilerPublishers map[string]workers.Publisher
}

func newNaisd(ctx context.Context, config testmanager.Config, db database.Repo) (*naisdRunner, func(), error) {
	naisdRunner := &naisdRunner{}
	pubsub := runner.NewPubSub(naisdRunner.doPublish)
	naisdRunner.PubSub = pubsub

	if v, _ := config.Bool("no_naisd"); v {
		return nil, func() {}, nil
	}

	if v, _ := config.Bool("no_tenants"); v {
		return nil, func() {}, nil
	}

	statusCh := make(chan pubsubMockMsg)
	mgmt, err := newNaisdForEnv(ctx.Done(), config, envManagementName, naisdRunner, statusCh)
	if err != nil {
		close(statusCh)
		return nil, func() {}, err
	}

	envTenant, err := newNaisdForEnv(ctx.Done(), config, envTenantName, naisdRunner, statusCh)
	if err != nil {
		close(statusCh)
		return nil, func() {}, err
	}

	envNonCI, err := newNaisdForEnv(ctx.Done(), config, envTenantNonCI, naisdRunner, statusCh)
	if err != nil {
		close(statusCh)
		return nil, func() {}, err
	}

	close := func() {
		close(statusCh)
	}
	// go func() {
	// 	for msg := range statusCh {
	// 		mp := make(map[string]any)
	// 		if err := json.Unmarshal(msg.msg, &mp); err != nil {
	// 			return
	// 		}

	// 		naisdRunner.Receive("status", runner.PubSubMessage{
	// 			Msg: mp,
	// 		})
	// 	}
	// }()

	statusMgr := &mockSubscriber[message.Status]{
		topic:    "status",
		messages: statusCh,
		done:     ctx.Done(),
		pubsub:   naisdRunner.PubSub,
	}

	log := logrus.New()
	// log.Out = os.Stdout
	// log.Level = logrus.DebugLevel
	log.Out = io.Discard
	rec := workers.NewReceiver(statusMgr, db, logrus.NewEntry(log))

	go mgmt.Run(ctx)
	go envTenant.Run(ctx)
	go envNonCI.Run(ctx)
	go rec.Run(ctx)

	return naisdRunner, close, nil
}

func newNaisdForEnv(done <-chan struct{}, config testmanager.Config, env string, naisdRunner *naisdRunner, statusCh chan pubsubMockMsg) (*naisd.DeployManager, error) {
	reconCh := make(chan pubsubMockMsg)
	reconPublisher := &mockPublisher[message.DeployInstruction]{
		topic:    "naisd-" + tenantName + "-" + env,
		pubsub:   naisdRunner.PubSub,
		messages: reconCh,
	}
	naisdRunner.registerReconcilerPublisher("naisd-"+tenantName+"-"+env, reconPublisher)

	deploySubscriber := &mockSubscriber[message.DeployInstruction]{
		topic:    "naisd-" + tenantName + "-" + env,
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
	naisdRunner.registerTopic("status", statusCh)

	logr := logrus.New()
	logr.Level = logrus.DebugLevel
	logr.Out = os.Stdout
	logr.Out = io.Discard

	var returnError error
	if v, _ := config.Bool("fail_execution"); v {
		returnError = fmt.Errorf("execution failed")
	}

	return naisd.NewDeployManager(
		deploySubscriber,
		statusPublisher,
		tenantName,
		env,
		&naisd.MockExecutor{Logger: logrus.NewEntry(logr), Timeout: 1 * time.Millisecond, ReturnError: returnError},
		nil,
		&rest.Config{},
		"",
		"",
		logrus.NewEntry(logr),
	)
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

func (n *naisdRunner) doPublish(topic string, msg runner.PubSubMessage) {
	fmt.Println("DO PUBLISH")
}
