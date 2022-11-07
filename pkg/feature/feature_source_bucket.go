package feature

import (
	"context"
	"path/filepath"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

var _ FeatureSource = (*FeatureSourceBucket)(nil)

type FeatureSourceBucket struct {
	bucket    *storage.BucketHandle
	callbacks []FeatureSourceUpdated
	log       logrus.FieldLogger

	lock     sync.Mutex
	features []Feature
}

func NewFeatureSourceBucket(ctx context.Context, bucket *storage.BucketHandle, log logrus.FieldLogger) (*FeatureSourceBucket, error) {
	f := &FeatureSourceBucket{
		bucket: bucket,
		log:    log,
	}

	if err := f.sync(ctx); err != nil {
		return nil, err
	}

	return f, nil
}

func (f *FeatureSourceBucket) sync(ctx context.Context) error {
	f.log.Debug("syncing features")

	features := []Feature{}
	it := f.bucket.Objects(ctx, nil)
	for {
		objAttrs, err := it.Next()
		if err != nil && err != iterator.Done {
			return err
		}
		if err == iterator.Done {
			break
		}
		if filepath.Ext(objAttrs.Name) != ".yaml" {
			continue
		}

		r, err := f.bucket.Object(objAttrs.Name).NewReader(ctx)
		if err != nil {
			return err
		}
		defer r.Close()

		feat, err := parseFeature(objAttrs.Name, r)
		if err != nil {
			return err
		}

		features = append(features, feat)
	}

	f.lock.Lock()
	defer f.lock.Unlock()
	f.features = features
	return nil
}

func (f *FeatureSourceBucket) Features() ([]Feature, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	return f.features, nil
}

func (f *FeatureSourceBucket) Register(fn FeatureSourceUpdated) {
	f.callbacks = append(f.callbacks, fn)
}

func (f *FeatureSourceBucket) Watch(ctx context.Context, subscription *pubsub.Subscription) {
	ch := make(chan struct{}, 1)

	go func() {
		flushTimer := time.NewTicker(10 * time.Second)
		flushTimer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				flushTimer.Reset(10 * time.Second)
			case <-flushTimer.C:
				flushTimer.Stop()
				if err := f.sync(ctx); err != nil {
					f.log.WithError(err).Error("failed to sync features")
					continue
				}
				for _, fn := range f.callbacks {
					fn()
				}
			}
		}
	}()

	err := subscription.Receive(ctx, func(_ context.Context, msg *pubsub.Message) {
		f.log.Debug("received pubsub message")
		msg.Ack()
		ch <- struct{}{}
	})
	if err != nil {
		f.log.WithError(err).Error("failed to receive pubsub messages")
	}
}

func (f *FeatureSourceBucket) Close() error {
	return nil
}
