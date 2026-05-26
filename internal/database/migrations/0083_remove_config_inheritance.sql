-- +goose Up
ALTER TABLE configurations_environment NO INHERIT configurations_global;

-- +goose Down
ALTER TABLE configurations_environment INHERIT configurations_global;

