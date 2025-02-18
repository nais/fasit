package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"

	"github.com/nais/fasit/internal/integration_test/testmanager/parser"
)

type REST struct {
	server http.Handler
}

func NewRestRunner(server http.Handler) *REST {
	return &REST{server: server}
}

func (r *REST) Ext() string {
	return "rest"
}

func (r *REST) Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error {
	f, err := parser.Parse(body, state)
	if err != nil {
		return fmt.Errorf("rest.Parse: %w", err)
	}

	expectedResponseCode := f.Opts["responseCode"]

	delete(f.Opts, "responseCode")

	return f.Execute(state, func() (any, error) {
		rec := httptest.NewRecorder()

		parts := strings.SplitN(f.Query, "\n", 2)
		methodPath := strings.SplitN(strings.TrimSpace(parts[0]), " ", 2)

		logf("running %v request to %v", methodPath[0], methodPath[1])

		req, err := http.NewRequestWithContext(ctx, methodPath[0], methodPath[1], strings.NewReader(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("rest.Run: unable to create request: %w", err)
		}

		r.server.ServeHTTP(rec, req)

		if expectedResponseCode != "" {
			if strconv.Itoa(rec.Code) != expectedResponseCode {
				return nil, fmt.Errorf("expected response code %q, got %d\n%v", expectedResponseCode, rec.Code, rec.Body.String())
			}
		}

		res := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			return nil, fmt.Errorf("rest.Run: unable to unmarshal response: %w", err)
		}

		return res, nil
	})
}
