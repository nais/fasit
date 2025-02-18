package database

import (
	"fmt"
)

type ErrMissingRequiredFields struct {
	Fields []string
}

func (e *ErrMissingRequiredFields) Is(target error) bool {
	_, ok := target.(*ErrMissingRequiredFields)
	return ok
}

func (e *ErrMissingRequiredFields) Error() string {
	return fmt.Sprintf("missing required fields: %+v", e.Fields)
}
