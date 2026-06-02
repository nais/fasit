-- +goose Up
DROP RULE IF EXISTS audits_prevent_deletes ON audits;
DROP FUNCTION IF EXISTS raiseNoDeleteException();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION prevent_modify()
	RETURNS TRIGGER
	LANGUAGE plpgsql
	AS $$
BEGIN
	RAISE EXCEPTION 'Cannot modify rows in %', TG_TABLE_NAME;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audits_no_modify
	BEFORE DELETE OR UPDATE ON audits
	FOR EACH ROW
	EXECUTE FUNCTION prevent_modify();
