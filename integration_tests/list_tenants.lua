local tenantID = Helper.CreateTenant("tenant32", false)

Helper.CreateEnvironment(tenantID, "management", "management", true)
Helper.CreateEnvironment(tenantID, "testing", "tenant", true)
Helper.CreateEnvironment(tenantID, "nonci", "tenant")

Test.gql("List tenants with environment", function(t)
	t.query [[
		query {
			tenants {
				name
				environments {
					name
					kind
				}
			}
		}
	]]

	t.check({
		data = {
			tenants = {
				{
					name = "tenant32",
					environments = {
						{ name = "management", kind = "MANAGEMENT" },
						{ name = "nonci",      kind = "TENANT" },
						{ name = "testing",    kind = "TENANT" },
					}
				}
			}
		}
	})
end)
