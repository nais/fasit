local tenantID = Helper.CreateTenant("tenant1", true)
local testingID = Helper.CreateEnvironment(tenantID, "e1", "tenant", false, true)
Helper.CreateEnvironment(tenantID, "management", "management", false, true)
Helper.CreateEnvironment(tenantID, "nonci", "tenant")

Test.rest("create rollout v1", function(t)
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

Test.gql("no ci feature list", function(t)
	t.query [[
		{
			tenant(slug: "tenant1") {
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

Test.sql("verify tenant env", function(t)
	t.query [[
		SELECT
  			t.name AS tenant_name,
  			t.ci AS tenant_ci,
			e.name AS name,
  			e.ci AS ci
		FROM
			tenants t
			JOIN environments e ON e.tenant_id = t.id
		ORDER BY
			e.name
		;
	]]

	t.check(
		{
			{
				tenant_name = "tenant1",
				tenant_ci = true,
				name = "e1",
				ci = true,
			},
			{
				tenant_name = "tenant1",
				tenant_ci = true,
				name = "management",
				ci = true,
			},
			{
				tenant_name = "tenant1",
				tenant_ci = true,
				name = "nonci",
				ci = false,
			},
		}
	)
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

Test.pubsub("deploy instruction", function(t)
	Helper.Reconcile()

	t.check("naisd-tenant1-e1", {
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
						name = "e1",
					},
					tenant = {
						name = "tenant1",
					},
				},
				gcp = "true",
			},
			Version = "0.1.0-feature",
		},
	})
end)

Test.pubsub("naisd response", function(t)
	t.check("status", {
		attributes = Null,
		data = {
			Tenant = "tenant1",
			Environment = "e1",
			Type = 2,
			Data = NotNull(),
		},
	})
end)

Test.sql("verify rollout success", function(t)
	t.query [[
		SELECT
			COUNT(1)::float AS count
		FROM
			rollouts
		WHERE
			status = 'deployed'
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

Test.gql("no ci updated feature list", function(t)
	t.query [[
		{
			tenant(slug: "tenant1") {
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

Test.pubsub("new deploy instruction", function(t)
	Helper.Reconcile()

	t.check("naisd-tenant1-e1", {
		attributes = Null,
		data = {
			ConfigHash = NotNull(),
			ID = NotNull(),
			Chart = "oci://clamav",
			Name = "clamav",
			Timeout = 600000000000,
			Values = {
				fasit = {
					env = {
						kind = "tenant",
						name = "e1",
					},
					tenant = {
						name = "tenant1",
					},
				},
				gcp = "true",
			},
			Version = "0.1.1-feature",
		},
	})
end)

Test.pubsub("new naisd response", function(t)
	t.check("status", {
		attributes = Null,
		data = {
			Tenant = "tenant1",
			Environment = "e1",
			Type = 2,
			Data = NotNull(),
		},
	}
	)
end)

Test.gql("no ci another updated feature list", function(t)
	t.query [[
		{
			tenant(slug: "tenant1") {
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
						features = {
							{
								name = "clamav",
								version = "0.1.1-feature",
							},
						},
					},
				},
			},
		}
	)
end)
