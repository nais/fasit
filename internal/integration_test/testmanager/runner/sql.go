package runner

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nais/fasit/internal/integration_test/testmanager/parser"
)

var rowTypeRegexp = regexp.MustCompile(`(?m)^--\s*type:\s*row$`)

type SQL struct {
	db *pgxpool.Pool
}

func NewSQLRunner(db *pgxpool.Pool) *SQL {
	return &SQL{db: db}
}

func (s *SQL) Ext() string {
	return "sql"
}

func (s *SQL) Run(ctx context.Context, logf func(format string, args ...any), body []byte, state map[string]any) error {
	f, err := parser.Parse(body, state)
	if err != nil {
		return fmt.Errorf("gql.Parse: %w", err)
	}

	return f.Execute(state, func() (any, error) {
		if rowTypeRegexp.MatchString(f.Query) {
			return s.queryRow(ctx, f)
		}
		return s.query(ctx, f)
	})
}

func (s *SQL) query(ctx context.Context, f *parser.File) (any, error) {
	ret := []any{}
	err := s.db.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
		rows, err := c.Query(ctx, f.Query)
		if err != nil {
			return fmt.Errorf("sql.Run: unable to execute query: %w", err)
		}
		defer rows.Close()

		cols := rows.FieldDescriptions()
		for i := 0; rows.Next(); i++ {
			vals, err := rows.Values()
			if err != nil {
				return fmt.Errorf("sql.Run: unable to get values: %w", err)
			}

			row := map[string]any{}
			for i, col := range cols {
				row[string(col.Name)] = vals[i]
				if ui, ok := vals[i].([16]uint8); ok {
					row[string(col.Name)] = uuid.UUID(ui)
				} else {
					row[string(col.Name)] = vals[i]
				}
			}

			ret = append(ret, row)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sql.Run: unable to run query: %w", err)
	}

	return ret, nil
}

func (s *SQL) queryRow(ctx context.Context, f *parser.File) (any, error) {
	var ret map[string]any
	err := s.db.AcquireFunc(ctx, func(c *pgxpool.Conn) error {
		rows, err := c.Query(ctx, f.Query)
		if err != nil {
			return fmt.Errorf("sql.Run: unable to execute query: %w", err)
		}
		defer rows.Close()

		if !rows.Next() {
			if rows.Err() != nil {
				return rows.Err()
			}
			return pgx.ErrNoRows
		}

		vals, err := rows.Values()
		if err != nil {
			return fmt.Errorf("sql.Run: unable to get values: %w", err)
		}

		cols := rows.FieldDescriptions()
		row := map[string]any{}
		for i, col := range cols {
			row[string(col.Name)] = vals[i]
			if ui, ok := vals[i].([16]uint8); ok {
				row[string(col.Name)] = uuid.UUID(ui)
			} else {
				row[string(col.Name)] = vals[i]
			}
		}

		ret = row

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("sql.Run: unable to run query: %w", err)
	}

	return ret, nil
}
