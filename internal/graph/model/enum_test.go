package model

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"
	"testing"
)

type enum interface {
	comparable
	IsValid() bool
	String() string
	// UnmarshalGQL(v interface{}) error
	MarshalGQL(w io.Writer)
}

func testEnum[T enum](t *testing.T, valid []T, invalid []T) {
	t.Run("IsValid", func(t *testing.T) {
		for _, v := range valid {
			if !v.IsValid() {
				t.Errorf("expected %v to be valid", v)
			}
		}
		for _, v := range invalid {
			if v.IsValid() {
				t.Errorf("expected %v to be invalid", v)
			}
		}
	})

	t.Run("String", func(t *testing.T) {
		for _, v := range valid {
			if v.String() == "" {
				t.Errorf("expected %v to have a string representation", v)
			}
		}
		for _, v := range invalid {
			if v.String() == "" {
				t.Errorf("expected %v to not have a string representation", v)
			}
		}
	})

	t.Run("UnmarshalGQL", func(t *testing.T) {
		for _, v := range valid {
			res, err := unmarhsalWrapper[T](strings.ToUpper(v.String()))
			if err != nil {
				t.Errorf("%v: unexpected error: %v", v, err)
			}
			if res != v {
				t.Errorf("%v: expected %v, got %v", v, v, res)
			}
		}
		for _, v := range invalid {
			_, err := unmarhsalWrapper[T](strings.ToUpper(v.String()))
			if err == nil {
				t.Errorf("%v: expected error: %v", v, err)
			}
		}
	})

	t.Run("MarshalGQL", func(t *testing.T) {
		for _, v := range valid {
			var buf bytes.Buffer
			v.MarshalGQL(&buf)

			expected := fmt.Sprintf("%q", strings.ToUpper(v.String()))
			if buf.String() != expected {
				t.Errorf("expected %q, got %q", expected, buf.String())
			}
		}
		for _, v := range invalid {
			var buf bytes.Buffer
			v.MarshalGQL(&buf)
			expected := fmt.Sprintf("%q", strings.ToUpper(v.String()))
			if buf.String() != expected {
				t.Errorf("expected %q, got %q", expected, buf.String())
			}
		}
	})
}

func unmarhsalWrapper[T enum](v interface{}) (T, error) {
	var empty T
	target := new(T)

	res := reflect.ValueOf(target).MethodByName("UnmarshalGQL").Call([]reflect.Value{reflect.ValueOf(v)})
	if res[0].Interface() != nil {
		return empty, res[0].Interface().(error)
	}
	return *target, nil
}
