-- +goose Up
CREATE INDEX fasitd_commands_created_idx ON fasitd_commands(created);

-- +goose Down
DROP INDEX fasitd_commands_created_idx;

