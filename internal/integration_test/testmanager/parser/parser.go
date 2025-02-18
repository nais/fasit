package parser

import (
	"fmt"
	"strings"
)

const (
	NotNullOption = "NOTNULL"
	IgnoreOption  = "IGNORE"
)

func Parse(b []byte, state map[string]any) (*File, error) {
	f := &File{
		Opts:  make(map[string]string),
		Store: make(map[string]string),
	}

	str := removeComments(string(b))
	str, err := RenderTemplate(str, state)
	if err != nil {
		return nil, fmt.Errorf("parser.Parse: failed to render template: %w", err)
	}

	parts := strings.SplitN(str, "RETURNS", 2)
	f.Query = strings.TrimSpace(string(parts[0]))

	optParts := strings.SplitN(parts[1], "ENDOPTS", 2)
	returns := optParts[len(optParts)-1]

	if len(optParts) > 1 {
		os := strings.Split(optParts[0], "OPTION")
		for _, o := range os {
			if strings.TrimSpace(o) == "" {
				continue
			}
			ps := strings.SplitN(o, "=", 2)

			path := strings.TrimSpace(ps[0])
			option := strings.TrimSpace(ps[1])

			f.Opts[path] = option
		}
	}

	srs := strings.Split(returns, "STORE")
	if len(srs) > 1 {
		for _, s := range srs[1:] {
			sp := strings.Split(strings.TrimSpace(string(s)), "=")
			f.Store[strings.TrimSpace(sp[0])] = strings.TrimSpace(sp[1])
		}
	}

	f.Returns = strings.TrimSpace(string(srs[0]))

	return f, nil
}

func removeComments(s string) string {
	lines := strings.Split(s, "\n")
	ret := []string{}
	for _, l := range lines {
		if !strings.HasPrefix(l, "//") {
			ret = append(ret, l)
		}
	}
	return strings.Join(ret, "\n")
}
