-- Test cluster upgrade history at tenant and root level
local tenant1ID = Helper.CreateTenant("multi-level-tenant-1", false)
local tenant2ID = Helper.CreateTenant("multi-level-tenant-2", false)
local env1ID = Helper.CreateEnvironment(tenant1ID, "prod-env", "tenant", false, false)
local env2ID = Helper.CreateEnvironment(tenant1ID, "dev-env", "tenant", false, false)
local env3ID = Helper.CreateEnvironment(tenant2ID, "staging-env", "tenant", false, false)

-- Create cluster upgrades across multiple tenants and environments
-- Tenant 2, Environment 3: 4 upgrades (1.30.1-4) - hours 1-4 (newest)
for i = 1, 4 do
	local version = string.format("1.30.%d-gke.100", i)
	local hoursAgo = 5 - i -- 4, 3, 2, 1 hours ago
	Helper.SQLExec([[
		INSERT INTO cluster_upgrades (tenant_id, environment_id, version, status, start_time, last_modified)
		VALUES ($1, $2, $3, 'DONE', NOW() - INTERVAL ']] ..
		hoursAgo .. [[ hours', NOW() - INTERVAL ']] .. hoursAgo .. [[ hours')
	]], tenant2ID, env3ID, version)
end

-- Tenant 1, Environment 1: 3 upgrades (1.29.1-3) - hours 5-7
for i = 1, 3 do
	local version = string.format("1.29.%d-gke.100", i)
	local hoursAgo = 8 - i -- 7, 6, 5 hours ago
	Helper.SQLExec([[
		INSERT INTO cluster_upgrades (tenant_id, environment_id, version, status, start_time, last_modified)
		VALUES ($1, $2, $3, 'DONE', NOW() - INTERVAL ']] ..
		hoursAgo .. [[ hours', NOW() - INTERVAL ']] .. hoursAgo .. [[ hours')
	]], tenant1ID, env1ID, version)
end

-- Tenant 1, Environment 2: 2 upgrades (1.28.1-2) - hours 8-9 (oldest)
for i = 1, 2 do
	local version = string.format("1.28.%d-gke.100", i)
	local hoursAgo = 10 - i -- 9, 8 hours ago
	Helper.SQLExec([[
		INSERT INTO cluster_upgrades (tenant_id, environment_id, version, status, start_time, last_modified)
		VALUES ($1, $2, $3, 'DONE', NOW() - INTERVAL ']] ..
		hoursAgo .. [[ hours', NOW() - INTERVAL ']] .. hoursAgo .. [[ hours')
	]], tenant1ID, env2ID, version)
end

-- Test tenant-level query (should return upgrades from all environments in tenant1)
Test.gql("tenant level returns all environments in tenant", function(t)
	t.query(string.format([[
		{
			tenant(slug: "multi-level-tenant-1") {
				clusterUpgradeHistory {
					version
				}
			}
		}
	]]))

	-- Should return 5 records from tenant1 (3 from prod-env + 2 from dev-env)
	-- Ordered by last_modified DESC
	t.check({
		data = {
			tenant = {
				clusterUpgradeHistory = {
					{ version = "1.29.3-gke.100" },
					{ version = "1.29.2-gke.100" },
					{ version = "1.29.1-gke.100" },
					{ version = "1.28.2-gke.100" },
					{ version = "1.28.1-gke.100" },
				},
			},
		},
	})
end)

-- Test tenant-level query with limit
Test.gql("tenant level respects limit parameter", function(t)
	t.query(string.format([[
		{
			tenant(slug: "multi-level-tenant-1") {
				clusterUpgradeHistory(limit: 3) {
					version
				}
			}
		}
	]]))

	-- Should return only 3 newest records
	t.check({
		data = {
			tenant = {
				clusterUpgradeHistory = {
					{ version = "1.29.3-gke.100" },
					{ version = "1.29.2-gke.100" },
					{ version = "1.29.1-gke.100" },
				},
			},
		},
	})
end)

-- Test tenant-level query with offset
Test.gql("tenant level respects offset parameter", function(t)
	t.query(string.format([[
		{
			tenant(slug: "multi-level-tenant-1") {
				clusterUpgradeHistory(offset: 3) {
					version
				}
			}
		}
	]]))

	-- Should skip first 3 records and return remaining 2
	t.check({
		data = {
			tenant = {
				clusterUpgradeHistory = {
					{ version = "1.28.2-gke.100" },
					{ version = "1.28.1-gke.100" },
				},
			},
		},
	})
end)

-- Test root-level query (should return upgrades from all tenants and environments)
Test.gql("root level returns all tenants and environments", function(t)
	t.query([[
		{
			clusterUpgradeHistory {
				version
			}
		}
	]])

	-- Should return all 9 records (3+2 from tenant1 + 4 from tenant2)
	-- Ordered by last_modified DESC
	t.check({
		data = {
			clusterUpgradeHistory = {
				{ version = "1.30.4-gke.100" },
				{ version = "1.30.3-gke.100" },
				{ version = "1.30.2-gke.100" },
				{ version = "1.30.1-gke.100" },
				{ version = "1.29.3-gke.100" },
				{ version = "1.29.2-gke.100" },
				{ version = "1.29.1-gke.100" },
				{ version = "1.28.2-gke.100" },
				{ version = "1.28.1-gke.100" },
			},
		},
	})
end)

-- Test root-level query with limit
Test.gql("root level respects limit parameter", function(t)
	t.query([[
		{
			clusterUpgradeHistory(limit: 5) {
				version
			}
		}
	]])

	-- Should return only 5 newest records
	t.check({
		data = {
			clusterUpgradeHistory = {
				{ version = "1.30.4-gke.100" },
				{ version = "1.30.3-gke.100" },
				{ version = "1.30.2-gke.100" },
				{ version = "1.30.1-gke.100" },
				{ version = "1.29.3-gke.100" },
			},
		},
	})
end)

-- Test root-level query with offset
Test.gql("root level respects offset parameter", function(t)
	t.query([[
		{
			clusterUpgradeHistory(offset: 5, limit: 3) {
				version
			}
		}
	]])

	-- Should skip first 5 records and return next 3
	t.check({
		data = {
			clusterUpgradeHistory = {
				{ version = "1.29.2-gke.100" },
				{ version = "1.29.1-gke.100" },
				{ version = "1.28.2-gke.100" },
			},
		},
	})
end)

-- Test that each tenant only sees their own data
Test.gql("tenant level only returns data for that tenant", function(t)
	t.query(string.format([[
		{
			tenant(slug: "multi-level-tenant-2") {
				clusterUpgradeHistory {
					version
				}
			}
		}
	]]))

	-- Should return only 4 records from tenant2
	t.check({
		data = {
			tenant = {
				clusterUpgradeHistory = {
					{ version = "1.30.4-gke.100" },
					{ version = "1.30.3-gke.100" },
					{ version = "1.30.2-gke.100" },
					{ version = "1.30.1-gke.100" },
				},
			},
		},
	})
end)
