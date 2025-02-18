package parser

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

func RenderTemplate(query string, state map[string]any) (string, error) {
	tpl, err := template.New("query").Funcs(sprig.TxtFuncMap()).Parse(query)
	if err != nil {
		return "", fmt.Errorf("unable to parse template: %w", err)
	}

	buf := &bytes.Buffer{}
	if err := tpl.Execute(buf, state); err != nil {
		return "", fmt.Errorf("unable to execute template: %w", err)
	}

	return buf.String(), nil
}
