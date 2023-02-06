package feature

type Dependencies []Dependency

func (d Dependencies) FindMissing(features []string) []string {
	ret := []string{}
	for _, dep := range d {
		ret = append(ret, dep.FindMissing(features)...)
	}
	return ret
}

type Dependency struct {
	Any []string `json:"any,omitempty" yaml:"any,omitempty"`
	All []string `json:"all,omitempty" yaml:"all,omitempty"`
}

func (d *Dependency) FindMissing(features []string) []string {
	contains := func(s []string, e string) bool {
		for _, a := range s {
			if a == e {
				return true
			}
		}
		return false
	}

	missing := []string{}
	if len(d.All) > 0 {
		for _, f := range d.All {
			if !contains(features, f) {
				missing = append(missing, f)
			}
		}
	}

	anyOfMissing := []string{}
	for _, f := range d.Any {
		if contains(features, f) {
			anyOfMissing = []string{}
			break
		}
		anyOfMissing = append(anyOfMissing, f)
	}
	return append(missing, anyOfMissing...)
}
