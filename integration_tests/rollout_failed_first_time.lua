local tenantID = Helper.CreateTenant("tenant23", true)
Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "testing", "tenant", false, true)
Helper.CreateEnvironment(tenantID, "nonci", "tenant")

Test.rest("create rollout", function(t)
	t.send("POST", "/github/rollout", [[
		{
			"chart": "oci://clamav",
			"version": "0.1.0-feature"
		}
	]])

	t.check(201, {
		id = NotNull(),
		envNotAvailable = {
			"tenant",
		},
	})
end)

Test.gql("enable feature", function(t)
	t.query(string.format([[
		mutation {
  			featureStateSave(
  				envID: "%s"
  				enabled: true
  				feature: "clamav"
			) {
    			id
    			enabled
  			}
		}
	]], testingID))

	t.check(
		{
			data = {
				featureStateSave = {
					id = NotNull(),
					enabled = true,
				},
			},
		}
	)
end)

Helper.NaisdEnvironmentFailing(testingID, true)

Test.pubsub("deploy instruction", function(t)
	Helper.Reconcile()

	t.check("naisd-tenant23-testing", {
		attributes = Null,
		data = {
			ConfigHash = NotNull(),
			ID = NotNull(),
			Chart = "oci://clamav",
			Name = "clamav",
			Timeout = 0,
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
			Version = "0.1.0-feature",
		},
	})
end)

Test.sql("verify rollout failure", function(t)
	t.query [[
		SELECT
			COUNT(1)::float AS count
		FROM
			rollouts
		WHERE
			status = 'failed'
		;
	]]
	t.check(
		{
			{
				count = 1,
			},
		}
	)
end)

Test.gql("no ci feature list", function(t)
	t.query [[
		{
			tenant(slug: "tenant23") {
				environment(slug: "nonci") {
					features {
						name
						version
					}
				}
			}
		}
	]]

	t.check(
		{
			data = {
				tenant = {
					environment = {
						features = {},
					},
				},
			},
		}
	)
end)

Test.gql("ci feature list", function(t)
	t.query [[
		{
			tenant(slug: "tenant23") {
				environment(slug: "testing") {
					features {
						name
						version
					}
				}
			}
		}
	]]

	t.check(
		{
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
	)
end)
