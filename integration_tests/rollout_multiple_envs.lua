local tenantID = Helper.CreateTenant("tenant23", true)
local managementID = Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "testing", "tenant", false, true)
Helper.CreateEnvironment(tenantID, "nonci", "tenant")

Helper.SQLExec("UPDATE environments SET reconcile = false WHERE id = $1", managementID)

Test.rest("create rollout", function(t)
	t.send("POST", "/github/rollout", [[
	{
		"chart": "oci://allenvs",
		"version": "1.0.0"
	}
	]])

	t.check(201, {
		id = NotNull(),
		envNotAvailable = { "tenant", "management", "legacy", "onprem" },
	})
end)

Test.gql("enable features", function(t)
	t.query(string.format([[
		mutation {
			featureStateSave(envID: "%s", enabled: true, feature: "allenvs") {
				enabled
			}
			mgmt : featureStateSave(envID: "%s", enabled: true, feature: "allenvs") {
				enabled
			}
		}
	]], testingID, managementID))

	t.check(
		{
			data = {
				featureStateSave = {
					enabled = true,
				},
				mgmt = {
					enabled = true,
				},
			},
		})
end)

Test.sql("verify deploy instructions", function(t)
	Helper.Reconcile();

	t.queryRow("SELECT count(1)::float AS count FROM deploy_instructions WHERE status = 'deployed';")

	t.check({ count = 1 })
end)

Test.sql("verify rollout success with management", function(t)
	Helper.Reconcile();
	t.queryRow("SELECT count(1)::float AS count FROM rollouts WHERE status = 'deployed';")


	t.check { count = 0 }
end)

Test.gql("enable reconcile of management", function(t)
	t.query(string.format([[
		mutation {
			environmentSetReconcile(id: "%s", reconcile: true) {
				reconcile
			}
		}
	]], managementID))

	t.check {
		data = {
			environmentSetReconcile = {
				reconcile = true,
			},
		},
	}
end)

Test.sql("verify deploy instructions with management", function(t)
	Helper.Reconcile();
	local managementDI = Helper.SQLQueryRow([[
		SELECT id::text, environment_id::text
		FROM deploy_instructions
		WHERE feature_name = 'allenvs'
		AND environment_id = $1
		]], managementID)


	t.queryRow("SELECT status FROM deploy_instructions WHERE id = $1", managementDI.id)

	t.check { status = "deployed" }
end)

Test.sql("verify rollout success with management", function(t)
	Helper.Reconcile();
	t.queryRow([[
		SELECT count(1)::float AS count
		FROM rollouts
		WHERE status = 'deployed'
			AND feature_name = 'allenvs';
	]])

	t.check { count = 1 }
end)
