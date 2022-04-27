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
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Description  *string   `json:"description"`
	Created      time.Time `json:"created"`
	LastModified time.Time `json:"lastModified"`
}

type EnvironmentKind string

const (
	EnvironmentKindPartner    EnvironmentKind = "partner"
	EnvironmentKindManagement EnvironmentKind = "management"
)

var AllEnvironmentKind = []EnvironmentKind{
	EnvironmentKindPartner,
	EnvironmentKindManagement,
}

func (e EnvironmentKind) IsValid() bool {
	switch e {
	case EnvironmentKindPartner, EnvironmentKindManagement:
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
