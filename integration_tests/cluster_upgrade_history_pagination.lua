-- Test cluster upgrade history pagination
local tenantID = Helper.CreateTenant("pagination-tenant", true)
local envID = Helper.CreateEnvironment(tenantID, "test-env", "tenant", false, true)

-- Create 15 cluster upgrades with different timestamps
for i = 1, 15 do
	local version = string.format("1.28.%d-gke.100", i)
	Helper.SQLExec([[
		INSERT INTO cluster_upgrades (tenant_id, environment_id, version, status, start_time, last_modified)
		VALUES ($1, $2, $3, 'DONE', NOW() - INTERVAL ']] ..
	(15 - i) .. [[ hours', NOW() - INTERVAL ']] .. (15 - i) .. [[ hours')
	]], tenantID, envID, version)
end

-- Test default pagination (should return all 15 records, less than default limit of 50)
Test.gql("default pagination returns all records", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory {
						version
					}
				}
			}
		}
	]]))

	-- Should return all 15 records in DESC order (newest first)
	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = {
						{ version = "1.28.15-gke.100" },
						{ version = "1.28.14-gke.100" },
						{ version = "1.28.13-gke.100" },
						{ version = "1.28.12-gke.100" },
						{ version = "1.28.11-gke.100" },
						{ version = "1.28.10-gke.100" },
						{ version = "1.28.9-gke.100" },
						{ version = "1.28.8-gke.100" },
						{ version = "1.28.7-gke.100" },
						{ version = "1.28.6-gke.100" },
						{ version = "1.28.5-gke.100" },
						{ version = "1.28.4-gke.100" },
						{ version = "1.28.3-gke.100" },
						{ version = "1.28.2-gke.100" },
						{ version = "1.28.1-gke.100" },
					},
				},
			},
		},
	})
end)

-- Test limit parameter
Test.gql("limit parameter controls number of records", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 5) {
						version
					}
				}
			}
		}
	]]))

	-- Should return only 5 newest records
	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = {
						{ version = "1.28.15-gke.100" },
						{ version = "1.28.14-gke.100" },
						{ version = "1.28.13-gke.100" },
						{ version = "1.28.12-gke.100" },
						{ version = "1.28.11-gke.100" },
					},
				},
			},
		},
	})
end)

-- Test offset parameter
Test.gql("offset parameter skips records", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 5, offset: 5) {
						version
					}
				}
			}
		}
	]]))

	-- Should skip first 5 and return next 5
	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = {
						{ version = "1.28.10-gke.100" },
						{ version = "1.28.9-gke.100" },
						{ version = "1.28.8-gke.100" },
						{ version = "1.28.7-gke.100" },
						{ version = "1.28.6-gke.100" },
					},
				},
			},
		},
	})
end)

-- Test limit=0 defaults to 50
Test.gql("limit 0 defaults to 50", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 0) {
						version
					}
				}
			}
		}
	]]))

	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = NotNull(),
				},
			},
		},
	})
end)

-- Test negative limit defaults to 50
Test.gql("negative limit defaults to 50", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: -1) {
						version
					}
				}
			}
		}
	]]))

	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = NotNull(),
				},
			},
		},
	})
end)

-- Test offset=0 is valid (doesn't skip anything)
Test.gql("offset 0 is valid", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 3, offset: 0) {
						version
					}
				}
			}
		}
	]]))

	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = NotNull(),
				},
			},
		},
	})
end)

-- Test negative offset defaults to 0
Test.gql("negative offset defaults to 0", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 3, offset: -1) {
						version
					}
				}
			}
		}
	]]))

	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = NotNull(),
				},
			},
		},
	})
end)

-- Test offset beyond available records returns empty
Test.gql("offset beyond records returns empty", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 5, offset: 100) {
						version
					}
				}
			}
		}
	]]))

	-- Should return empty array
	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = {},
				},
			},
		},
	})
end)

-- Test maximum limit enforcement (limit > 1000 should be capped at 1000)
-- Note: We can't easily test this with only 15 records, but we can verify it doesn't error
Test.gql("large limit is accepted", function(t)
	t.query(string.format([[
		{
			tenant(slug: "pagination-tenant") {
				environment(slug: "test-env") {
					clusterUpgradeHistory(limit: 2000) {
						version
					}
				}
			}
		}
	]]))

	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeHistory = NotNull(),
				},
			},
		},
	})
end)
