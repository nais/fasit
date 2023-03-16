package feature

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/nais/fasit/pkg/graph/model"
	"gopkg.in/yaml.v2"
)

var templateFuncs = template.FuncMap{
	"mapOf":             mapOf,
	"mapJoin":           mapJoin,
	"environmentsAsMap": environmentsAsMap,
	"prefixedValues":    prefixedValues,
	"subdomain":         subdomain,
	"eachOf":            eachOf,
	"toJSON":            toJSON,
	"fromJSON":          fromJSON,
	"toYAML":            toYAML,
	"join":              join,
	"filter":            filter,
	"replace":           replace,
	"b64enc":            base64encode,
}

// mapOf creates a new map from a list of map[string]any with the given key as the key in the new map,
// and the given valueKey as the value in the new map.
func mapOf(key, valueKey string, m []map[string]any) map[string]any {
	ret := map[string]any{}
	for _, v := range m {
		key, okKey := v[key]
		value, okValue := v[valueKey]
		if okKey && okValue {
			ret[key.(string)] = value
		}
	}
	return ret
}

// replace replaces all occurrences of the given old string with the given new string.
func replace(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

// mapJoin joins the values of a map into a string, with the given separator.
func mapJoin(sep string, m any) []string {
	typ := reflect.TypeOf(m)
	if typ.Kind() != reflect.Map {
		panic(fmt.Sprintf("mapJoin: expected map, got %T", m))
	}

	val := reflect.ValueOf(m)
	keys := val.MapKeys()
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].String() < keys[j].String()
	})

	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = fmt.Sprintf("%s%s%s", k, sep, val.MapIndex(k))
	}
	return out
}

// prefixedValues returns a map of all values in the given map that have the given prefix.
func prefixedValues(m any, prefix string) []any {
	switch m := m.(type) {
	case map[string]any:
		out := []any{}
		for k, v := range m {
			if strings.HasPrefix(k, prefix) {
				out = append(out, v)
			}
		}

		sort.Slice(out, func(i, j int) bool {
			return fmt.Sprint(out[i]) < fmt.Sprint(out[j])
		})
		return out
	default:
		return []any{prefix}
	}
}

// subdomain returns the subdomain for the active environment.
func subdomain(m *MappingValues, subdomain string) string {
	domain := m.Tenant.Name + ".cloud.nais.io"
	if m.Kind != model.EnvironmentKindManagement {
		domain = m.Env["name"].(string) + "." + domain
	}
	return subdomain + "." + domain
}

// Return relevant values from .Envs[] in a custom structure.
//
// INPUT:
//
//	[
//	  { name: "mycluster"
//	  , value1: ...
//	  , value2: ...
//	  }
//	]
//
// OUTPUT:
//
//	{
//	   mycluster: {
//	     value1: ...,
//	     value2: ...,
//	   }
//	}
//
// USAGE:
//
// environmentsAsMap "value1,value2" .Envs
// .
func environmentsAsMap(keyList string, data []map[string]any) map[string]map[string]any {
	const primaryKey = "name"
	keys := strings.Split(keyList, ",")
	result := make(map[string]map[string]any)
	for _, envMap := range data {
		key, ok := envMap[primaryKey].(string)
		if !ok {
			panic(fmt.Sprintf("trying to use env[%s] as string key, but it is a %T of value %q",
				primaryKey, envMap[primaryKey], envMap[primaryKey]))
		}
		if key == "" {
			panic(fmt.Sprintf("env[%s] is empty, but it needs to be used as a primary key", primaryKey))
		}
		result[key] = make(map[string]any)
		for k, v := range envMap {
			for _, includedKey := range keys {
				if k == includedKey {
					result[key][k] = v
					break
				}
			}
		}
	}
	return result
}

// eachOf returns a list of values by iterating over the given slice and getting the value using the given key.
// the slice must be a slice of maps or structs.
func eachOf(m any, key string) []any {
	ret := []any{}
	typ := reflect.TypeOf(m)
	if typ.Kind() != reflect.Slice {
		panic(fmt.Sprintf("eachOf: expected slice or array, got %T", m))
	}
	val := reflect.ValueOf(m)

	switch typ.Elem().Kind() {
	case reflect.Map:
		for i := 0; i < val.Len(); i++ {
			v := val.Index(i).Interface().(map[string]any)
			if v, ok := v[key]; ok {
				ret = append(ret, v)
			}
		}
	case reflect.Struct:
		for i := 0; i < val.Len(); i++ {
			ret = append(ret, val.Index(i).FieldByName(key).Interface())
		}
	default:
		panic(fmt.Sprintf("eachOf: expected map or struct, got %T", m))
	}

	return ret
}

// toJSON returns the JSON representation of the given value.
func toJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// fromJSON returns a map of the given JSON string.
func fromJSON(s string) map[string]any {
	b := []byte(s)
	m := map[string]any{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		panic(err)
	}
	return m
}

// toYAML returns the YAML representation of the given value.
func toYAML(v any) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// join joins the given values into a string, with the given separator.
func join(sep string, v any) string {
	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Slice {
		panic(fmt.Sprintf("join: expected slice or array, got %T", v))
	}

	val := reflect.ValueOf(v)
	out := make([]string, val.Len())
	for i := 0; i < val.Len(); i++ {
		out[i] = fmt.Sprintf("%v", val.Index(i))
	}
	return strings.Join(out, sep)
}

func filter(key string, value, v any) []map[string]any {
	typ := reflect.TypeOf(v)
	if typ.Kind() != reflect.Slice {
		panic(fmt.Sprintf("filter: expected slice or array, got %T", v))
	}

	val := reflect.ValueOf(v)
	out := make([]map[string]any, 0)
	for i := 0; i < val.Len(); i++ {
		mp := val.Index(i).Interface().(map[string]any)
		if mp[key] == value {
			out = append(out, mp)
		}
	}
	return out
}

func base64encode(v any) string {
	if v == nil {
		return ""
	}

	s, ok := v.(string)
	if !ok {
		panic("expected string to b64enc")
	}

	return base64.StdEncoding.EncodeToString([]byte(s))
}
