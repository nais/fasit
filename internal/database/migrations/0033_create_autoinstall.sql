-- +goose Up
CREATE TABLE auto_installs(
	kind environment_kind NOT NULL,
	feature TEXT NOT NULL REFERENCES features(NAME) ON DELETE CASCADE,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

