-- name: EnvironmentDeleteLabels :exec
DELETE FROM environment_labels
WHERE
	environment_id = @environment_id
;

-- name: EnvironmentInsertLabels :batchexec
INSERT INTO
	environment_labels ("environment_id", "key", "value")
VALUES
	(@environment_id, @key, @value)
;

-- name: EnvironmentGetLabels :many
SELECT * FROM environment_labels
WHERE environment_id = @environment_id
ORDER BY "key"
;
