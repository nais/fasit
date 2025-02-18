-- +goose Up
ALTER TABLE tenants
ADD CONSTRAINT tenants_name_unique UNIQUE ("name");

ALTER TABLE environments
ADD CONSTRAINT environments_name_unique UNIQUE ("tenant_id", "name");
