-- +goose Up
CREATE TABLE rollouts (
	id uuid PRIMARY KEY DEFAULT uuid_generate_v4 (),
	feature_name TEXT NOT NULL UNIQUE,
	version TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending',
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"completed" TIMESTAMPTZ DEFAULT NULL,
	FOREIGN KEY (feature_name, version) REFERENCES feature_data (name, version)
)
;
