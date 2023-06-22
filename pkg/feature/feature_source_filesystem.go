package feature

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

var _ FeatureSource = (*FeatureSourceFilesystem)(nil)

type FeatureSourceFilesystem struct {
	log       logrus.FieldLogger
	directory string
	callbacks []FeatureSourceUpdated
	watcher   *fsnotify.Watcher
}

func NewFeatureSourceFilesystem(directory string, log logrus.FieldLogger) (*FeatureSourceFilesystem, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = watcher.Add(directory)
	if err != nil {
		return nil, err
	}

	return &FeatureSourceFilesystem{
		log:       log,
		watcher:   watcher,
		directory: directory,
	}, nil
}

func (f *FeatureSourceFilesystem) Register(callback FeatureSourceUpdated) {
	f.callbacks = append(f.callbacks, callback)
}

func (f *FeatureSourceFilesystem) Features() ([]Feature, error) {
	features := []Feature{}
	err := filepath.WalkDir(f.directory, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".yaml" {
			return nil
		}

		f, err := os.OpenFile(path, os.O_RDONLY, 0)
		if err != nil {
			return err
		}
		defer f.Close()

		feature, err := parseFeature(path, f)
		if err != nil {
			return fmt.Errorf("%v: %w", path, err)
		}

		features = append(features, feature)

		return nil
	})

	return features, err
}

func (f *FeatureSourceFilesystem) Watch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-f.watcher.Events:
			if !ok {
				f.log.Error("filesystem watcher closed (events)")
				return
			}

			if filepath.Ext(event.Name) != ".yaml" {
				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				for _, callback := range f.callbacks {
					callback()
				}
			}
		case err, ok := <-f.watcher.Errors:
			if !ok {
				f.log.Error("filesystem watcher closed (errors)")
				return
			}
			log.Println("error:", err)
		}
	}
}

func (f *FeatureSourceFilesystem) Close() error {
	return f.watcher.Close()
}
