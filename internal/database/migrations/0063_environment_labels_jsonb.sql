-- +goose Up
ALTER TABLE environments
ADD COLUMN IF NOT EXISTS labels JSONB NOT NULL DEFAULT '{}'::JSONB
;

UPDATE environments e
SET
	labels = el.labels
FROM
	(
		SELECT
			environment_id,
			JSONB_OBJECT_AGG(key, value) AS labels
		FROM
			environment_labels
		GROUP BY
			environment_id
	) el
WHERE
	e.id = el.environment_id
;

DROP TABLE IF EXISTS environment_labels
;
