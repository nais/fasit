-- +goose Up
CREATE TABLE audits(
	id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
	actor TEXT NOT NULL,
	description TEXT NOT NULL,
	object_type TEXT NOT NULL,
	object_id TEXT NOT NULL,
	created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX audits_object_id_idx ON audits USING spgist(object_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION raiseNoDeleteException()
	RETURNS VOID
	LANGUAGE plpgsql
	VOLATILE
	AS $$
BEGIN
	RAISE EXCEPTION 'Cannot delete row.';
END
$$;

-- +goose StatementEnd
CREATE OR REPLACE RULE audits_prevent_deletes AS ON DELETE TO audits
	DO INSTEAD
	SELECT
		raiseNoDeleteException();

