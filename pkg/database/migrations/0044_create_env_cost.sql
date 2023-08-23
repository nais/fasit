-- +goose Up

CREATE TABLE env_cost (
  tenant_id UUID NOT NULL,
  env_id UUID NOT NULL,
  date DATE NOT NULL,
  cost real NOT NULL,
  PRIMARY KEY (tenant_id, env_id, date),
  CONSTRAINT fk_env_cost_tenant
      FOREIGN KEY (tenant_id)
          REFERENCES tenants (id) ON DELETE CASCADE,
  CONSTRAINT fk_env_cost_env
      FOREIGN KEY (env_id)
          REFERENCES environments (id) ON DELETE CASCADE
);
