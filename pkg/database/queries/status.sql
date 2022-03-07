-- name: StatusCreateOrUpdate :exec
INSERT INTO status (environment_id, feature, version, status)
VALUES (@environmentID, @feature, @version, @status)
ON CONFLICT (environment_id, feature)
DO
UPDATE SET version=EXCLUDED.version, status=EXCLUDED.status;
