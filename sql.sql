WITH "inner" AS (
		SELECT
			id,
			environment_id,
			key,
			value,
			created,
			(CASE WHEN environment_id IS NULL THEN 1 ELSE 0 END) as env,
			rank()
		OVER (PARTITION BY key ORDER BY created DESC)
		FROM configurations
		WHERE feature = 'naiserator'
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
