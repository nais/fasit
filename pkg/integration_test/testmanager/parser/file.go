package parser

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
)

type File struct {
	Query   string
	Opts    map[string]string
	Returns string
	Store   map[string]string
}

func (f *File) CmpOpts() ([]cmp.Option, error) {
	cmpopts := []cmp.Option{}
	for path, option := range f.Opts {
		switch option {
		case NotNullOption:
			cmpopts = append(cmpopts, cmp.FilterPath(ignorePath(path), cmp.Comparer(cmpNotNull)))
		case IgnoreOption:
			cmpopts = append(cmpopts, cmp.FilterPath(ignorePath(path), cmp.Ignore()))
		default:
			return nil, fmt.Errorf("unknown option %q for path %q", option, path)
		}
	}
	return cmpopts, nil
}

func (f *File) Execute(state map[string]any, ex func() (any, error)) error {
	val, err := ex()
	if err != nil {
		return err
	}
	if val, ok := val.(map[string]any); ok {
		f.AppendStore(val, state)
	}

	var expected any
	if strings.HasPrefix(f.Returns, "[") {
		expected = []any{}
	} else {
		expected = map[string]any{}
	}
	if err := json.Unmarshal([]byte(f.Returns), &expected); err != nil {
		return fmt.Errorf("unable to unmarshal expected returns: %w\n%v", err, f.Returns)
	}

	cmpopts, err := f.CmpOpts()
	if err != nil {
		return err
	}
	if !cmp.Equal(val, expected, cmpopts...) {
		return fmt.Errorf("diff -want +got:\n%v", cmp.Diff(expected, val, cmpopts...))
	}
	return nil
}

func (f *File) AppendStore(val map[string]any, state map[string]any) {
	for key, path := range f.Store {
		var (
			root = val
			val  interface{}
		)

		pathParts := strings.Split(path, ".")

		for i := 0; i < len(pathParts); i++ {
			if i == len(pathParts)-1 {
				// Last element of pathParts
				val = root[pathParts[i]]
				break
			} else if i < len(pathParts)-2 {
				// check if next value is a list
				intVal, err := strconv.Atoi(pathParts[i+1])
				if err == nil {
					slice := root[pathParts[i]].([]interface{})
					root = slice[intVal].(map[string]interface{})
					// skip one iteration on lists
					i += 1
					continue
				}
			}
			root = root[pathParts[i]].(map[string]interface{})
		}
		state[key] = val
	}
}

func ignorePath(path string) func(p cmp.Path) bool {
	return func(p cmp.Path) bool {
		s := ""
		wide := ""
		for _, pe := range p {
			switch pe := pe.(type) {
			case cmp.MapIndex:
				s += "." + pe.Key().String()
				wide += "." + pe.Key().String()
			case cmp.SliceIndex:
				s += "." + strconv.Itoa(pe.Key())
				wide += ".*"
			}
		}
		return s == "."+path || wide == "."+path
	}
}

func cmpNotNull(a, b interface{}) bool {
	if a == nil || b == nil {
		return false
	}
	return true
}
