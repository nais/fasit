package model

import "testing"

func TestEnvironmentKind(t *testing.T) {
	testEnum(t, AllEnvironmentKind, []EnvironmentKind{"dev", "prod"})
}
