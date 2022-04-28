package model

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Environment struct {
	ID           uuid.UUID       `json:"id"`
	Name         string          `json:"name"`
	Description  *string         `json:"description"`
	Created      time.Time       `json:"created"`
	LastModified time.Time       `json:"lastModified"`
	Kind         EnvironmentKind `json:"kind"`
}

type EnvironmentKind string

const (
	EnvironmentKindTenant     EnvironmentKind = "tenant"
	EnvironmentKindManagement EnvironmentKind = "management"
)

var AllEnvironmentKind = []EnvironmentKind{
	EnvironmentKindTenant,
	EnvironmentKindManagement,
}

func (e EnvironmentKind) IsValid() bool {
	switch e {
	case EnvironmentKindTenant, EnvironmentKindManagement:
		return true
	}
	return false
}

func (e EnvironmentKind) String() string {
	return string(e)
}

func (e *EnvironmentKind) UnmarshalGQL(v interface{}) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	str = strings.ToLower(str)

	*e = EnvironmentKind(str)
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid EnvironmentKind", str)
	}
	return nil
}

func (e EnvironmentKind) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(strings.ToUpper(e.String())))
}
