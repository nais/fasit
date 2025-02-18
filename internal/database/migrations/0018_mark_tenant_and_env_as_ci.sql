-- +goose Up

ALTER TABLE tenants
  ADD COLUMN "ci" BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE environments
  ADD COLUMN "ci" BOOLEAN NOT NULL DEFAULT FALSE;

-- Only allow one tenant to be marked as CI
CREATE UNIQUE INDEX ON "tenants" ("ci")
WHERE "ci" = true;

-- Only allow one environment to be marked as CI
CREATE UNIQUE INDEX ON "environments" ("ci", "kind")
WHERE "ci" = true;
