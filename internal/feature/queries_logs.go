package feature

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/hashicorp/go-multierror"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/nais/fasit/internal/feature/featuresql"
	"github.com/nais/fasit/internal/graph/model"
	"github.com/nais/fasit/internal/message"
)

func LogCreate(ctx context.Context, deployInstructionID uuid.UUID, lines []message.LogLine) error {
	params := make([]featuresql.LogsCreateParams, len(lines))
	for i, line := range lines {
		params[i] = featuresql.LogsCreateParams{
			DeployInstruction: deployInstructionID,
			Time: pgtype.Timestamptz{
				Time:  line.Time,
				Valid: true,
			},
			Message: line.Msg,
			Kind:    string(line.Kind),
		}
	}

	br := querier(ctx).LogsCreate(ctx, params)

	var outerErr error
	br.Exec(func(i int, err error) {
		if err != nil {
			outerErr = multierror.Append(outerErr, err)
		}
	})

	return outerErr
}

func LogsGet(ctx context.Context, deployInstructionID uuid.UUID) ([]*model.LogLine, error) {
	logs, err := querier(ctx).LogsByDeployInstruction(ctx, deployInstructionID)
	if err != nil {
		return nil, err
	}

	logLines := make([]*model.LogLine, len(logs))
	for i, log := range logs {
		logLines[i] = logLineFromSQL(log)
	}

	return logLines, nil
}

func LogsByID(ctx context.Context, id int) (*model.LogLine, error) {
	log, err := querier(ctx).LogsByID(ctx, int64(id))
	if err != nil {
		return nil, err
	}

	return logLineFromSQL(log), nil
}

func logLineFromSQL(log featuresql.Log) *model.LogLine {
	return &model.LogLine{
		ID:                  fmt.Sprintf("%s-%d", log.DeployInstruction, log.ID),
		Timestamp:           log.Time.Time,
		Message:             log.Message,
		IntID:               int(log.ID),
		DeployInstructionID: log.DeployInstruction,
	}
}
