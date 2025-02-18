package database

// trigger := func(ctx context.Context, id uuid.UUID) {
// 	ch <- struct{}{}
// }

// go func() {
// 	if err := r.repo.RolloutsListen(ctx, trigger); err != nil {
// 		r.log.WithError(err).Error("rollouts listen")
// 	}
// }()

// go func() {
// 	if err := r.repo.FeatureStatesListen(ctx, trigger); err != nil {
// 		r.log.WithError(err).Error("feature states listen")
// 	}
// }()

// return r.repo.ConfigListen(ctx, trigger)
