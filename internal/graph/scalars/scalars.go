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
		if id == uuid.Nil {
			_, _ = w.Write([]byte("null"))
			return
		}
		b, _ := json.Marshal(id)
		_, _ = w.Write(b)
	})
}

func UnmarshalUUID(v any) (uuid.UUID, error) {
	switch v := v.(type) {
	case string:
		return uuid.Parse(v)
	default:
		return uuid.Nil, fmt.Errorf("%T is not a string", v)
	}
}

func MarshalRawMessage(msg json.RawMessage) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		_, _ = w.Write(msg)
	})
}

func UnmarshalRawMessage(v any) (json.RawMessage, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func MarshalEnvironmentLabels(val map[string]string) graphql.Marshaler {
	return graphql.WriterFunc(func(w io.Writer) {
		if err := json.NewEncoder(w).Encode(val); err != nil {
			panic(err)
		}
	})
}

func UnmarshalEnvironmentLabels(v any) (map[string]string, error) {
	if m, ok := v.(map[string]string); ok {
		return m, nil
	}

	return nil, fmt.Errorf("%T is not a map[string]string", v)
}
