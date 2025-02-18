package model

import "testing"

func TestConditionStatus(t *testing.T) {
	testEnum(t, AllConditionStatus, []ConditionStatus{"other", "random"})
}

// This can't be tested through testEnum atm because of the way the enum is defined.
// func TestKubernetesNodeConditionType(t *testing.T) {
// 	testEnum(t, AllKubernetesNodeConditionType, []KubernetesNodeConditionType{"other", "random"})
// }
