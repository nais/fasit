local tenantID = Helper.CreateTenant("tenant23", true)
local managementID = Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "testing", "tenant", false, true)
local nonciID = Helper.CreateEnvironment(tenantID, "nonci", "tenant")

Test.rest("create rollout", function(t)
	t.send("POST", "/github/rollout", [[
	{
		"chart": "oci://clamav",
		"version": "0.1.0-feature"
	}
	]])

	t.check(201, {
		id = NotNull(),
		envNotAvailable = { "tenant" },
	})
end)

Test.gql("enable feature", function(t)
	t.query(string.format([[
		mutation {
			featureStateSave(envID: "%s", enabled: true, feature: "clamav") {
				id
				enabled
			}
		}
	]], testingID))

	t.check {
		data = {
			featureStateSave = {
				id = NotNull(),
				enabled = true,
			},
		},
	}
end)

Test.sql("verify deploy instructions", function(t)
	Helper.Reconcile();

	t.queryRow("SELECT count(1)::float AS count FROM deploy_instructions WHERE status = 'deployed';")

	t.check({ count = 1 })
end)

Helper.emptyPubSubTopic("naisd-tenant23-testing")
Helper.emptyPubSubTopic("status")
Helper.NaisdEnvironmentFailing(testingID)

Test.rest("create new rollout", function(t)
	t.send("POST", "/github/rollout", [[
	{
		"chart": "oci://clamav",
		"version": "0.1.1-feature"
	}
	]])

	t.check(201, {
		id = NotNull(),
		envNotAvailable = {},
	})
end)

Test.pubsub("new deployment instruction", function(t)
	Helper.Reconcile()

	t.check("naisd-tenant23-testing", {
		attributes = Null,
		data = {
			ID = NotNull(),
			ConfigHash = NotNull(),
			Chart = "oci://clamav",
			Name = "clamav",
			Timeout = 600000000000,
			Values = {
				fasit = {
					env = {
						kind = "tenant",
						name = "testing",
					},
					tenant = {
						name = "tenant23",
					},
				},
				gcp = "true",
			},
			Version = "0.1.1-feature",
		},
	})
end)

Test.pubsub("naisd response", function(t)
	t.check("status", {
		attributes = Null,
		data = {
			Tenant      = "tenant23",
			Environment = "testing",
			Type        = 2,
			Data        = NotNull(),
		},
	})
end)


Test.sql("verify rollout", function(t)
	Helper.Reconcile();

	t.queryRow("SELECT status FROM rollouts WHERE feature_name = 'clamav' AND version = '0.1.1-feature';")

	t.check({ status = "failed" })
end)

Test.gql("no ci feature list", function(t)
	t.query([[
		{
			tenant(slug: "tenant23") {
				environment(slug: "nonci") {
					features {name, version}
				}
			}
		}
	]])

	t.check {
		data = {
			tenant = {
				environment = {
					features = {
						{
							name = "clamav",
							version = "0.1.0-feature",
						},
					},
				},
			},
		},
	}
end)
