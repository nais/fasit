-- +goose Up
DELETE FROM environment_values
WHERE
	key = 'slack_upgrade_mentions';

-- +goose Down
-- Data is not recoverable after deletion.
