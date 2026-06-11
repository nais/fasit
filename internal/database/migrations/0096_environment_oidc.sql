-- +goose Up
-- OIDC issuer and discovery URL per environment. Used to validate projected
-- Kubernetes service account tokens that fasitd agents present to Fasit. The
-- discovery URL may differ from the issuer claim when discovery is fetched via
-- a reverse proxy (e.g. on-prem apiservers exposed through oidcproxy).
ALTER TABLE environments
	ADD COLUMN oidc_issuer TEXT,
	ADD COLUMN oidc_discovery_url TEXT;

-- +goose Down
ALTER TABLE environments
	DROP COLUMN oidc_issuer,
	DROP COLUMN oidc_discovery_url;

