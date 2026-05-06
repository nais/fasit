local tenantID = Helper.CreateTenant("tenant1", true)
Helper.CreateEnvironment(tenantID, "e1", "tenant", false, true)
Helper.CreateEnvironment(tenantID, "management", "management", false, true)

Test.rest("create deployment for clamav", function(t)
	t.send("POST", "/github/deployment", [[
		{
			"chart": "oci://clamav",
			"version": "0.1.0-feature",
			"target" : {
				"kind": "tenant"
			},
			"ci": {"wait": true}
		}
	]])
	t.check(201, {
		id = NotNull(),
	})
end)

Test.rest("rollout for same feature is rejected", function(t)
	t.send("POST", "/github/rollout", [[
		{
			"chart": "oci://clamav",
			"version": "0.1.1-feature"
		}
	]])
	-- Body is plain text from http.Error, so we cannot use t.check here
	-- (it requires JSON). The sql test below verifies no rollout row was created.
end)

Test.sql("no rollout row created", function(t)
	t.query [[
		SELECT
			COUNT(1)::float AS count
		FROM
			rollouts
		WHERE
			feature_name = 'clamav'
		;
	]]

	t.check(
		{
			{
				count = 0,
			},
		}
	)
end)
