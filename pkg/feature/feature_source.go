package feature

type FeatureSourceUpdated func()

type FeatureSource interface {
	Features() ([]Feature, error)
	Register(FeatureSourceUpdated)
	Close() error
}
