-- +goose Up
CREATE TYPE environment_kind AS ENUM('tenant', 'management')
;

CREATE TABLE environments (
	"id" uuid DEFAULT uuid_generate_v4 (),
	"tenant_id" uuid NOT NULL,
	"name" TEXT NOT NULL,
	"kind" environment_kind NOT NULL DEFAULT 'tenant',
	"description" TEXT,
	"created" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	"last_modified" TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (id),
	CONSTRAINT fk_environments_tenant FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE CASCADE
)
;

CREATE TRIGGER environments_set_modified BEFORE
UPDATE ON environments FOR EACH ROW
EXECUTE PROCEDURE update_modified_timestamp ()
;
