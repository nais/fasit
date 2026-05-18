package types

import "fmt"

type EnvironmentLabels map[string]string

type EnvironmentKind string

const (
	EnvironmentKindTenant     EnvironmentKind = "tenant"
	EnvironmentKindManagement EnvironmentKind = "management"
	EnvironmentKindOnprem     EnvironmentKind = "onprem"
	EnvironmentKindLegacy     EnvironmentKind = "legacy"
)

func (e *EnvironmentKind) Scan(src interface{}) error {
	switch s := src.(type) {
	case []byte:
		*e = EnvironmentKind(s)
	case string:
		*e = EnvironmentKind(s)
	default:
		return fmt.Errorf("unsupported scan type for EnvironmentKind: %T", src)
	}
	return nil
}
