-- +goose Up
CREATE TABLE audits (
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor TEXT NOT NULL,
    description text NOT NULL,
    object_type text NOT NULL,
    object_id text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX audits_object_id_idx  ON audits USING spgist(object_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION raiseNoDeleteException()
RETURNS VOID LANGUAGE plpgsql volatile AS $$ BEGIN
    raise exception 'Cannot delete row.';
END$$;
-- +goose StatementEnd

CREATE OR REPLACE RULE audits_prevent_deletes AS
    ON DELETE TO audits DO INSTEAD SELECT raiseNoDeleteException()
;
