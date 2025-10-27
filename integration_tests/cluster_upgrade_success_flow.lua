-- Test successful cluster upgrade flow with actual Go code execution
local tenantID = Helper.CreateTenant("cluster-tenant-1", true)
local envID = Helper.CreateEnvironment(tenantID, "production", "tenant", false, true)

-- Set up environment values for cluster upgrade
Helper.SQLExec([[
    INSERT INTO environment_values (environment_id, key, value, secret)
    VALUES ($1, 'project_id', '"test-project-123"', false)
]], envID)

Helper.SQLExec([[
    INSERT INTO environment_values (environment_id, key, value, secret)
    VALUES ($1, 'slack_upgrade_mentions', '"<@U123456> <@channel>"', false)
]], envID)

Helper.SQLExec([[
    INSERT INTO environment_values (environment_id, key, value, secret)
    VALUES ($1, 'auto_upgrade', 'true', false)
]], envID)

-- Test GraphQL query to get initial cluster upgrade status (should be none)
Test.gql("no initial cluster upgrade", function(t)
	t.query(string.format([[
		{
			tenant(slug: "cluster-tenant-1") {
				environment(slug: "production") {
					clusterUpgradeStatus {
						id
						upgradeStatus
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
					clusterUpgradeStatus = Null,
				},
			},
		},
	})
end)

-- Test manual cluster upgrade trigger via GraphQL mutation
Test.gql("trigger cluster upgrade via mutation", function(t)
	t.query(string.format([[
		mutation {
			environmentUpgrade(upgrade: {
				envID: "%s"
				version: "1.28.5-gke.1234567"
			}) {
				id
				name
				clusterUpgradeStatus {
					id
					upgradeStatus
					version
				}
			}
		}
	]], envID))

	t.check({
		data = {
			environmentUpgrade = {
				id = envID,
				name = "production",
				clusterUpgradeStatus = {
					id = NotNull(),
					upgradeStatus = "CREATED",
					version = "1.28.5-gke.1234567",
				},
			},
		},
	})
end)

-- Test that database reflects the triggered upgrade
Test.sql("verify upgrade in database", function(t)
	t.query([[
        SELECT
            version,
            status,
            tenant_id::text as tenant_id,
            environment_id::text as environment_id
        FROM cluster_upgrades
        WHERE tenant_id = $1 AND environment_id = $2
    ]], tenantID, envID)

	t.check({
		{
			version = "1.28.5-gke.1234567",
			status = "CREATED",
			tenant_id = tenantID,
			environment_id = envID,
		},
	})
end)

-- Test GraphQL query shows upgrade status
Test.gql("verify upgrade status via GraphQL", function(t)
	t.query([[
		{
			tenant(slug: "cluster-tenant-1") {
				environment(slug: "production") {
					clusterUpgradeStatus {
						upgradeStatus
						version
					}
				}
			}
		}
	]])

	t.check({
		data = {
			tenant = {
				environment = {
					clusterUpgradeStatus = {
						upgradeStatus = "CREATED",
						version = "1.28.5-gke.1234567",
					},
				},
			},
		},
	})
end)
