package reconciler

import "context"

type ctxKey struct{}

// WithContext returns a new context with the reconciler stored in it.
func WithContext(ctx context.Context, r *Reconciler) context.Context {
	return context.WithValue(ctx, ctxKey{}, r)
}

// FromContext returns the reconciler stored in the context, or nil.
func FromContext(ctx context.Context) *Reconciler {
	r, _ := ctx.Value(ctxKey{}).(*Reconciler)
	return r
}
