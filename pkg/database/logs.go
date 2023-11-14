package database

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/pkg/database/gensql"
	"github.com/nais/fasit/pkg/graph/model"
	"github.com/nais/fasit/pkg/message"
)

type LogRepo interface {
	LogCreate(ctx context.Context, deployInstructionID uuid.UUID, lines []message.LogLine) error
	LogsGet(ctx context.Context, deployInstructionID uuid.UUID) ([]*model.LogLine, error)
	LogsByID(ctx context.Context, id int) (*model.LogLine, error)
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
			Kind:    string(line.Kind),
		}
	}

	br := r.querier.LogsCreate(ctx, params)

	var outerErr error
	br.Exec(func(i int, err error) {
		if err != nil {
			outerErr = multierror.Append(outerErr, err)
		}
	})

	return outerErr
}

func (r *repo) LogsGet(ctx context.Context, deployInstructionID uuid.UUID) ([]*model.LogLine, error) {
	logs, err := r.querier.LogsByDeployInstruction(ctx, deployInstructionID)
	if err != nil {
		return nil, err
	}

	logLines := make([]*model.LogLine, len(logs))
	for i, log := range logs {
		logLines[i] = logLineFromSQL(log)
	}

	return logLines, nil
}

func (r *repo) LogsByID(ctx context.Context, id int) (*model.LogLine, error) {
	log, err := r.querier.LogsByID(ctx, int64(id))
	if err != nil {
		return nil, err
	}

	return logLineFromSQL(log), nil
}

func logLineFromSQL(log gensql.Log) *model.LogLine {
	return &model.LogLine{
		ID:                  fmt.Sprintf("%s-%d", log.DeployInstruction, log.ID),
		Timestamp:           log.Time.Time,
		Message:             log.Message,
		IntID:               int(log.ID),
		DeployInstructionID: log.DeployInstruction,
	}
}
