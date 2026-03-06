package model

import "slices"

type Dependency struct {
	AnyOf []string `json:"anyOf,omitempty" yaml:"anyOf,omitempty"`
	AllOf []string `json:"allOf,omitempty" yaml:"allOf,omitempty"`
}

type Dependencies []*Dependency

func (d Dependencies) FindMissing(features []string) []string {
	ret := []string{}
	for _, dep := range d {
		ret = append(ret, dep.FindMissing(features)...)
	}
	return ret
}

func (d *Dependency) FindMissing(features []string) []string {
	contains := func(s []string, e string) bool {
		return slices.Contains(s, e)
	}

	missing := []string{}
	if len(d.AllOf) > 0 {
		for _, f := range d.AllOf {
			if !contains(features, f) {
				missing = append(missing, f)
			}
		}
	}

	anyOfMissing := []string{}
	for _, f := range d.AnyOf {
		if contains(features, f) {
			anyOfMissing = []string{}
			break
		}
		anyOfMissing = append(anyOfMissing, f)
	}
	return append(missing, anyOfMissing...)
}
