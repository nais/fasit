-- +goose Up
CREATE TABLE env_cost(
	tenant_id UUID NOT NULL,
	env_id UUID NOT NULL,
	date DATE NOT NULL,
	cost REAL NOT NULL,
	PRIMARY KEY (tenant_id, env_id, DATE),
	CONSTRAINT fk_env_cost_tenant FOREIGN KEY (tenant_id) REFERENCES tenants(id) ON DELETE CASCADE,
	CONSTRAINT fk_env_cost_env FOREIGN KEY (env_id) REFERENCES environments(id) ON DELETE CASCADE
);

