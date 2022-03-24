-- name: ConfigGet :many
SELECT *
FROM configurations
WHERE feature = @feature AND environment_id = uuid_nil();

-- name: ConfigUpdateOrCreate :one
INSERT INTO configurations
	(environment_id, feature, description, secret, key, value)
VALUES
	(@environment_id, @feature, @description, @secret, @key, @value)
ON CONFLICT (environment_id, feature, key) DO UPDATE
	SET
		value = EXCLUDED.value,
		description = EXCLUDED.description
RETURNING *;

-- name: ConfigGetForEnv :many
SELECT *
FROM configurations
WHERE feature = @feature AND environment_id = @environment_id;

-- name: ConfigForEnv :many
WITH "inner" AS (
		SELECT
			id,
			environment_id,
			key,
			value,
			created,
			(CASE WHEN environment_id = uuid_nil() THEN 1 ELSE 0 END) as env,
			rank()
		OVER (PARTITION BY key ORDER BY created DESC)
		FROM configurations
		WHERE feature = @feature
		AND (environment_id = uuid_nil() OR environment_id = @environment_id::uuid)
	),
	"outer" AS (
	SELECT
		id,
		environment_id,
		key,
		value,
		created,
		env,
		rank()
	OVER (PARTITION BY key ORDER BY env ASC, "inner".rank ASC)
	FROM "inner"
)
SELECT * FROM "outer" WHERE rank = 1;


-- name: EnvConfig :many
WITH "inner" AS (
    SELECT
        id,
        environment_id,
        feature,
        description,
        secret,
    key,
    value,
    created,
   (CASE WHEN environment_id = uuid_nil() THEN 0 ELSE 1 END)::boolean as env,
    rank()
    OVER (PARTITION BY key ORDER BY created ASC)
FROM configurations
WHERE feature = @feature
  AND (environment_id  = @environment_id OR environment_id = uuid_nil())
    ),
    "outer" AS (
SELECT
    id,
    environment_id,
    feature,
    description,
    secret,
    key,
    value,
    created,
    env,
    rank()
    OVER (PARTITION BY key ORDER BY env DESC, "inner".rank ASC)
FROM "inner"
    )
SELECT * FROM "outer" WHERE rank = 1;

-- name: ConfigUpdate :one
UPDATE configurations
SET description = @description,
	value = @value
WHERE id = @id
RETURNING *;

-- name: ConfigDelete :exec
DELETE FROM configurations
WHERE id = @id;
