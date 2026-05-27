package components

import (
	"context"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

type DataLoader[T any] func(ctx context.Context) ([]T, error)
type Renderer[T any] func(env T) g.Node

func Error(err error) g.Node {
	return h.Div(h.Class("error"), h.P(g.Text(err.Error())))
}

func Map[T any](ctx context.Context, dl DataLoader[T], r Renderer[T]) g.Node {
	items, err := dl(ctx)
	if err != nil {
		return Error(err)
	}

	return g.Map(items, r)
}
