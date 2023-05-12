package feature

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
)

var _ FeatureSource = (*FeatureSourceFilesystem)(nil)

type FeatureSourceFilesystem struct {
	log       logrus.FieldLogger
	directory string
	callbacks []FeatureSourceUpdated
}

func NewFeatureSourceFilesystem(directory string) (*FeatureSourceFilesystem, error) {
	return &FeatureSourceFilesystem{
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
			return err
		}

		features = append(features, feature)

		return nil
	})

	return features, err
}

func (f *FeatureSourceFilesystem) Watch(ctx context.Context) {
}

func (f *FeatureSourceFilesystem) Close() error {
	return nil
}
