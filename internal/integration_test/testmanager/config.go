package testmanager

type Config map[string]any

func (c Config) Bool(key string) (value bool, ok bool) {
	v, ok := c[key]
	if !ok {
		return false, false
	}
	value, ok = v.(bool)
	if !ok {
		return false, false
	}
	return value, true
}

func (c Config) Int(key string) (value int, ok bool) {
	v, ok := c[key]
	if !ok {
		return 0, false
	}
	f, ok := v.(float64)
	if !ok {
		return 0, false
	}
	return int(f), true
}
