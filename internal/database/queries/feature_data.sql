-- name: FeatureDataCreate :exec
INSERT INTO
	feature_data (
		name,
		version,
		chart,
		description,
		source,
		kinds,
		dependencies,
		VALUES
,
			default_values,
			timeout,
			tpl_details,
		RENAME
	)
VALUES
	(
		@feature_name,
		@version,
		@chart,
		@description,
		@source,
		(@kinds::TEXT[])::environment_kind[],
		@dependencies,
		@values,
		@default_values,
		@timeout,
		@tpl_details,
		@rename
	)
;
