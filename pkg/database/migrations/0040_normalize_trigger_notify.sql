-- +goose Up

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION fasit_notify() RETURNS trigger AS $$
BEGIN
  -- We accept a number of keys as arguments, and will read the values using NEW if it is set, or OLD if it is not.
  -- We will then send a notification to fasit_notify with a JSON object containing the keys and values, as well as
  -- the table name and operation.
  DECLARE
    values text[];
    i integer := 0;
    key text;
  BEGIN
    IF TG_NARGS > 0 THEN
      FOREACH key IN ARRAY TG_ARGV LOOP
        IF NEW IS NOT NULL THEN
          values := array_append(values, row_to_json(NEW)->>key);
        ELSE
          values := array_append(values, row_to_json(OLD)->>key);
        END IF;
        i := i + 1;
      END LOOP;
    END IF;

    -- Construct the JSON object and send the notification. The JSON object will be of the form:
    -- {
    --   "table": "table_name",
    --   "op": "operation",
    --   "data": {
    --     "key1": "value1",
    --     "key2": "value2",
    --     ...
    --   }
    -- }
    PERFORM pg_notify('fasit_notify', jsonb_build_object('table', TG_TABLE_NAME, 'op', TG_OP, 'data', jsonb_object(TG_ARGV, values))::text);
    RETURN NULL;
  END;
RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS configurations_global_notify ON configurations_global;
DROP TRIGGER IF EXISTS configurations_environment_notify ON configurations_environment;
DROP TRIGGER IF EXISTS feature_states_notify ON feature_states;
DROP TRIGGER IF EXISTS rollouts_notify ON rollouts;

CREATE TRIGGER configurations_global_notify
  AFTER INSERT OR UPDATE
  ON configurations_global
  FOR EACH ROW
  EXECUTE PROCEDURE fasit_notify("id", "feature");

CREATE TRIGGER configurations_environment_notify
  AFTER INSERT OR UPDATE
  ON configurations_environment
  FOR EACH ROW
  EXECUTE PROCEDURE fasit_notify("id", "feature", "environment_id");

CREATE TRIGGER feature_states_notify
  AFTER INSERT OR UPDATE
  ON feature_states
  FOR EACH ROW
  EXECUTE PROCEDURE fasit_notify("environment_id", "feature", "enabled");

CREATE TRIGGER rollouts_notify
  AFTER INSERT
  ON rollouts
  FOR EACH ROW
  EXECUTE PROCEDURE fasit_notify("id", "feature_name", "status");
