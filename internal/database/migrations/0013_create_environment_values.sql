-- +goose Up
CREATE TABLE environment_values (
	"environment_id" uuid NOT NULL REFERENCES environments (id) ON DELETE CASCADE,
	"key" TEXT NOT NULL,
	"value" JSONB NOT NULL,
	PRIMARY KEY ("environment_id", "key")
)
;
