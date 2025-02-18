local tenantID = Helper.CreateTenant("tenant23", true)
local managementID = Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "testing", "tenant", false, true)
local nonciID = Helper.CreateEnvironment(tenantID, "nonci", "tenant")

Test.rest("create rollout v1", function(t)
	t.send("POST", "/github/rollout", [[
	{
		"chart": "oci://rename",
		"version": "1.0"
	}
]])

	t.check(201, {
		id = Save("rollout_id"),
		envNotAvailable = { "tenant", "management" }
	})
end)
