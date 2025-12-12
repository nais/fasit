-- Test cluster upgrade bypass delay functionality
local tenantID = Helper.CreateTenant("bypass-delay-tenant", true)
local envID = Helper.CreateEnvironment(tenantID, "production", "tenant", false, true)

-- Set up environment values for cluster upgrade
Helper.SQLExec([[
    INSERT INTO environment_values (environment_id, key, value, secret)
    VALUES ($1, 'project_id', '"test-project-bypass"', false)
]], envID)

-- Create an upgrade in WAITING status
Helper.SQLExec(string.format([[
    INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, is_automatic)
    VALUES ('11111111-1111-1111-1111-111111111111', '%s', '%s', '1.29.0-gke.1234567', 'WAITING', true)
]], tenantID, envID))

local waitingUpgradeID = "11111111-1111-1111-1111-111111111111"

-- Test bypassing a WAITING upgrade via GraphQL mutation
Test.gql("bypass delay on WAITING upgrade", function(t)
	t.query(string.format([[
		mutation {
			clusterUpgradeBypassDelay(upgradeID: "%s") {
				id
				upgradeStatus
				version
				isAutomatic
			}
		}
	]], waitingUpgradeID))

	t.check({
		data = {
			clusterUpgradeBypassDelay = {
				id = waitingUpgradeID,
				upgradeStatus = "CREATED",
				version = "1.29.0-gke.1234567",
				isAutomatic = false,
			},
		},
	})
end)

-- Verify the database reflects the changes
Test.sql("verify bypass in database", function(t)
	t.query([[
        SELECT
            status,
            is_automatic
        FROM cluster_upgrades
        WHERE id = $1
    ]], waitingUpgradeID)

	t.check({
		{
			status = "CREATED",
			is_automatic = false,
		},
	})
end)

-- Update the existing upgrade to DONE to avoid unique constraint
Helper.SQLExec(string.format([[
    UPDATE cluster_upgrades
    SET status = 'DONE'
    WHERE id = '%s'
]], waitingUpgradeID))

-- Create an upgrade in CREATED status for error testing
Helper.SQLExec(string.format([[
    INSERT INTO cluster_upgrades (id, tenant_id, environment_id, version, status, is_automatic)
    VALUES ('22222222-2222-2222-2222-222222222222', '%s', '%s', '1.29.1-gke.7654321', 'CREATED', true)
]], tenantID, envID))

local createdUpgradeID = "22222222-2222-2222-2222-222222222222"

-- Test bypassing a non-WAITING upgrade returns error
Test.gql("bypass delay on CREATED upgrade fails", function(t)
	t.query(string.format([[
		mutation {
			clusterUpgradeBypassDelay(upgradeID: "%s") {
				id
				upgradeStatus
			}
		}
	]], createdUpgradeID))

	t.check({
		data = Null,
		errors = {
			{
				message = Contains("cannot bypass delay: upgrade is in 'CREATED' status"),
				path = Ignore(),
			},
		},
	})
end)

-- Test bypassing non-existent upgrade returns error
Test.gql("bypass delay on non-existent upgrade fails", function(t)
	t.query([[
		mutation {
			clusterUpgradeBypassDelay(upgradeID: "00000000-0000-0000-0000-000000000000") {
				id
				upgradeStatus
			}
		}
	]])

	t.check({
		data = Null,
		errors = {
			{
				message = Contains("no rows in result set"),
				path = Ignore(),
			},
		},
	})
end)

-- Verify CREATED upgrade was NOT modified
Test.sql("verify CREATED upgrade unchanged", function(t)
	t.query([[
        SELECT
            status,
            is_automatic
        FROM cluster_upgrades
        WHERE id = $1
    ]], createdUpgradeID)

	t.check({
		{
			status = "CREATED",
			is_automatic = true,
		},
	})
end)
