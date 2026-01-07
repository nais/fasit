-- +goose Up
CREATE TABLE health_statuses(
	"environment_id" UUID,
	"reported_at" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (environment_id),
	CONSTRAINT fk_health_statuses_environments FOREIGN KEY (environment_id) REFERENCES environments(id) ON DELETE CASCADE
);

