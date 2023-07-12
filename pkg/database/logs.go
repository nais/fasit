package database

import (
	"context"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/message"
)

type LogRepo interface {
	LogCreate(ctx context.Context, deployInstructionID uuid.UUID, lines []message.LogLine) error
}

func (r *repo) LogCreate(ctx context.Context, deployInstructionID uuid.UUID, lines []message.LogLine) error {
	params := make([]gensql.LogsCreateParams, len(lines))
	for i, line := range lines {
		params[i] = gensql.LogsCreateParams{
			DeployInstruction: deployInstructionID,
			Time: pgtype.Timestamptz{
				Time:  line.Time,
				Valid: true,
			},
			Message: line.Msg,
		}
	}

	br := r.querier.LogsCreate(ctx, params)

	var err error
	br.Exec(func(i int, err error) {
		err = multierror.Append(err, err)
	})

	return err
}
