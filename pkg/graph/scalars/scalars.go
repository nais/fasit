package graph

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
	"github.com/nais/fasit/pkg/graph/model"
)

func MarshalID(id model.ID) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		b, _ := json.Marshal(uuid.UUID(id))
		w.Write(b)
	})
}

func UnmarshalID(v any) (model.ID, error) {
	switch v := v.(type) {
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			return model.ID{}, err
		}
		return model.ID(id), nil
	default:
		return model.ID{}, fmt.Errorf("%T is not a string", v)
	}
}

func MarshalRawMessage(msg json.RawMessage) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		w.Write(msg)
	})
}

func UnmarshalRawMessage(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}
