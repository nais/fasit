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

func (e *ConfigType) UnmarshalGQL(v interface{}) error {
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

type Configuration interface {
	IsConfiguration()
	SetType(ConfigType)
	SetDisplayName(string)
	SetDescription(string)
	GetKey() string
}

type EnvConfiguration struct {
	ID          uuid.UUID       `json:"id"`
	Description string          `json:"description"`
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Secret      bool            `json:"secret"`
	Created     time.Time       `json:"created"`
	Type        ConfigType      `json:"type"`
	DisplayName string          `json:"displayName"`
	RolloutID   *uuid.UUID      `json:"rolloutID"`

	EnvironmentID uuid.UUID
	FeatureName   string
}

func (EnvConfiguration) IsConfiguration()           {}
func (e *EnvConfiguration) SetType(t ConfigType)    { e.Type = t }
func (e *EnvConfiguration) GetKey() string          { return e.Key }
func (e *EnvConfiguration) SetDisplayName(n string) { e.DisplayName = n }
func (e *EnvConfiguration) SetDescription(n string) { e.Description = n }

type GlobalConfiguration struct {
	ID          uuid.UUID       `json:"id"`
	Description string          `json:"description"`
	Key         string          `json:"key"`
	Value       json.RawMessage `json:"value"`
	Secret      bool            `json:"secret"`
	Created     time.Time       `json:"created"`
	Type        ConfigType      `json:"type"`
	DisplayName string          `json:"displayName"`

	FeatureName string
}

func (GlobalConfiguration) IsConfiguration()           {}
func (g *GlobalConfiguration) SetType(t ConfigType)    { g.Type = t }
func (g *GlobalConfiguration) GetKey() string          { return g.Key }
func (g *GlobalConfiguration) SetDisplayName(n string) { g.DisplayName = n }
func (g *GlobalConfiguration) SetDescription(n string) { g.Description = n }
