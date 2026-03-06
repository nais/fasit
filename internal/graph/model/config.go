package model

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type NewConfiguration struct {
	EnvironmentID *uuid.UUID      `json:"environmentID"`
	Feature       string          `json:"feature"`
	Description   *string         `json:"description"`
	Key           string          `json:"key"`
	Value         json.RawMessage `json:"value"`
	Secret        bool
	RolloutID     *uuid.UUID
}

type ConfigType string

const (
	ConfigTypeString      ConfigType = "string"
	ConfigTypeInt         ConfigType = "int"
	ConfigTypeBool        ConfigType = "bool"
	ConfigTypeStringArray ConfigType = "string_array"
)

var AllConfigType = []ConfigType{
	ConfigTypeString,
	ConfigTypeInt,
	ConfigTypeBool,
	ConfigTypeStringArray,
}

func (e ConfigType) IsValid() bool {
	switch e {
	case ConfigTypeString, ConfigTypeInt, ConfigTypeBool, ConfigTypeStringArray:
		return true
	}
	return false
}

func (e ConfigType) String() string {
	return string(e)
}

func (e *ConfigType) UnmarshalGQL(v any) error {
	str, ok := v.(string)
	if !ok {
		return fmt.Errorf("enums must be strings")
	}

	*e = ConfigType(strings.ToLower(str))
	if !e.IsValid() {
		return fmt.Errorf("%s is not a valid ConfigType", str)
	}
	return nil
}

func (e ConfigType) MarshalGQL(w io.Writer) {
	fmt.Fprint(w, strconv.Quote(strings.ToUpper(e.String())))
}

func (e ConfigType) MarshalJSON() ([]byte, error) {
	return json.Marshal(strings.ToUpper(string(e)))
}

func (e *ConfigType) UnmarshalJSON(b []byte) error {
	s := ""
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*e = ConfigType(strings.ToLower(s))
	return nil
}

// Configurations contains configuration and computed values for one feature
type Configurations struct {
	FeatureName string     `json:"-"`
	EnvID       *uuid.UUID `json:"-"`
	RolloutID   uuid.UUID  `json:"-"`
}

type ConfigurationGraphVars struct {
	EnvironmentID *uuid.UUID
	FeatureName   string
}

type Configuration struct {
	ID      uuid.UUID       `json:"id"`
	Value   *Value          `json:"value"`
	Content json.RawMessage `json:"content"`
	Created time.Time       `json:"created"`
	Source  ConfigSource    `json:"source"`
	Key     string          `json:"key"`

	GraphVars ConfigurationGraphVars `json:"-" yaml:"-"`
}

func (c *Configuration) IsUpdate() {}
