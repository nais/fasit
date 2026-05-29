-- +goose Up
-- Add the column, defaulting to true for new rows going forward.
ALTER TABLE deployments
	ADD COLUMN active BOOLEAN NOT NULL DEFAULT TRUE;

-- Backfill: keep only the most recent deployment per (feature_name, target) as active.
-- All older historical rows are marked inactive.
UPDATE
	deployments
SET
	active = FALSE
WHERE
	id NOT IN ( SELECT DISTINCT ON (feature_name, target)
			id
		FROM
			deployments
		ORDER BY
			feature_name,
			target,
			created DESC);

-- Now safe to create: at most one active row per (feature_name, target).
CREATE UNIQUE INDEX deployments_one_active_per_feature_target ON deployments(feature_name, target)
WHERE
	active = TRUE;

-- +goose Down
DROP INDEX IF EXISTS deployments_one_active_per_feature_target;

ALTER TABLE deployments
	DROP COLUMN active;
