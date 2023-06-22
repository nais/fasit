package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/nais/fasit/pkg/integration_test/testmanager/parser"
)

type GQL struct {
	server http.Handler
}

func NewGQLRunner(server http.Handler) *GQL {
	return &GQL{server: server}
}

func (g *GQL) Ext() string {
	return "gql"
}

func (g *GQL) Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error {
	f, err := parser.Parse(body, state)
	if err != nil {
		return fmt.Errorf("gql.Parse: %w", err)
	}

	return f.Execute(state, func() (any, error) {
		rec := httptest.NewRecorder()

		b, err := json.Marshal(
			struct {
				OperationName *string                `json:"operationName"`
				Variables     map[string]interface{} `json:"variables"`
				Query         string                 `json:"query"`
			}{
				nil,
				map[string]interface{}{},
				f.Query,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("gql.Run: unable to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", "/", bytes.NewReader(b))
		if err != nil {
			return nil, fmt.Errorf("gql.Run: unable to create request: %w", err)
		}

		req.Header.Add("Content-Type", "application/json")

		g.server.ServeHTTP(rec, req)

		res := map[string]any{}
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			return nil, fmt.Errorf("gql.Run: unable to unmarshal response: %w", err)
		}

		if _, ok := res["errors"]; ok {
			return res, nil
		}

		return res, nil
	})
}
