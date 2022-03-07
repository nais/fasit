package graph

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/99designs/gqlgen/graphql"
	"github.com/google/uuid"
)

func MarshalUUID(id uuid.UUID) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		b, _ := json.Marshal(id)
		w.Write(b)
	})
}

func UnmarshalUUID(v any) (uuid.UUID, error) {
	switch v := v.(type) {
	case string:
		return uuid.Parse(v)
	default:
		return uuid.UUID{}, fmt.Errorf("%T is not a string", v)
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
