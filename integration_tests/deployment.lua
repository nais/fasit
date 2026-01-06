local tenantID = Helper.CreateTenant("tenant1", false)
Helper.CreateEnvironment(tenantID, "management", "management", false, true)
Helper.CreateEnvironment(tenantID, "nonci", "tenant", false, false, { kind = "tenant" })

Test.rest("create deployment", function(t)
	t.send("POST", "/github/deployment", [[
		{
			"chartUrl": "oci://clamav",
			"version": "0.1.0-feature",
			"target" : {
				"kind": "tenant"
			}
		}
	]])

	t.check(201, {
		id = NotNull(),
	})
end)

Test.rest("create second deployment", function(t)
	t.send("POST", "/github/deployment", [[
		{
			"chartUrl": "oci://allenvs",
			"version": "1.0.0",
			"target" : {
				"kind": "tenant"
			}
		}
	]])

	t.check(201, {
		id = NotNull(),
	})
end)

Test.gql("list deployments", function(t)
	t.query [[
		{
			deployments {
				id
				featureName
				version
				target {
				    key
				    value
				}
				created
			}
		}
	]]

	t.check(
		{
			data = {
				deployments = NotNull(),
			},
		}
	)
end)

Test.gql("list deployments by feature", function(t)
	t.query [[
		{
			deployments (feature: "clamav") {
				featureName
				version
				target {
				    key
				    value
				}
			}
		}
	]]

	t.check(
		{
			data = {
				deployments = {
					{
						featureName = "clamav",
						version = "0.1.0-feature",
						target = {
							{
								key = "kind",
								value = "tenant",
							},
						},
					},
				},
			},
		}
	)
end)
