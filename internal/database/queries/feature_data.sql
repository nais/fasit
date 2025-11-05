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
		"values",
		default_values,
		timeout,
		tpl_details,
		"rename"
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
