-- name: DeleteEnvironmentLabels :exec
DELETE FROM environment_labels
WHERE
	environment_id = @environment_id
;

-- name: InsertEnvironmentLabel :exec
INSERT INTO
	environment_labels ("environment_id", "key", "value")
VALUES
	(@environment_id, @key, @value)
;
