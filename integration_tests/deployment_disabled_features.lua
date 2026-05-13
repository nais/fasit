-- Verifies that disabled_features blocks the deployment reconciler:
-- a deployment to (env, feature) where a row exists in disabled_features
-- must end up with status DISABLED, while siblings without the row deploy
-- normally.

local tenantID = Helper.CreateTenant("tenant-disabled", false)
Helper.CreateEnvironment(tenantID, "management", "management", false, true, { kind = "management" })
Helper.CreateEnvironment(tenantID, "dev", "tenant", false, false, { kind = "tenant" })
local prodID = Helper.CreateEnvironment(tenantID, "prod", "tenant", false, false, { kind = "tenant" })

-- Disable clamav in prod before the deployment is created. The reconciler
-- looks at disabled_features at reconcile time, so the order of inserts
-- relative to CreateDeployment doesn't matter as long as the row is in
-- place before Reconcile() runs.
Helper.SQLExec([[
	INSERT INTO disabled_features (environment_id, feature)
	VALUES ($1, 'clamav')
]], prodID)

Test.rest("create deployment", function(t)
    t.send("POST", "/github/deployment", [[
		{
			"chart": "oci://clamav",
			"version": "0.1.0-feature",
			"target": {
				"kind": "tenant"
			},
			"ci": {"wait": true}
		}
	]])
    t.check(201, {
        id = Save("deploymentId"),
    })
end)

Helper.Reconcile()

Test.gql("deployment is DEPLOYED in dev and DISABLED in prod", function(t)
    t.query(string.format([[
		{
			deployment (id: "%s") {
				feature {
					name
					version
				}
				statuses {
					environment {
						name
					}
					state
					message
				}
			}
		}
	]], State.deploymentId))

    t.check(
        {
            data = {
                deployment = {
                    feature = {
                        name = "clamav",
                        version = "0.1.0-feature",
                    },
                    statuses = {
                        {
                            environment = { name = "dev" },
                            state = "DEPLOYED",
                            message = "received status from naisd.",
                        },
                        {
                            environment = { name = "prod" },
                            state = "DISABLED",
                            message = "feature is disabled in this environment",
                        },
                    },
                },
            },
        }
    )
end)

-- Re-enable by removing the row, deploy a new version, and confirm both
-- environments now reach DEPLOYED. Guards against the disabled state
-- being sticky once cleared.
Helper.SQLExec([[
	DELETE FROM disabled_features
	WHERE environment_id = $1 AND feature = 'clamav'
]], prodID)

Test.rest("create follow-up deployment after re-enable", function(t)
    t.send("POST", "/github/deployment", [[
		{
			"chart": "oci://clamav",
			"version": "0.1.1-feature",
			"target": {
				"kind": "tenant"
			},
			"ci": {"wait": true}
		}
	]])
    t.check(201, {
        id = Save("followupDeploymentId"),
    })
end)

Helper.Reconcile()

Test.gql("follow-up deployment is DEPLOYED in both dev and prod", function(t)
    t.query(string.format([[
		{
			deployment (id: "%s") {
				statuses {
					environment {
						name
					}
					state
				}
			}
		}
	]], State.followupDeploymentId))

    t.check(
        {
            data = {
                deployment = {
                    statuses = {
                        {
                            environment = { name = "prod" },
                            state = "DEPLOYED",
                        },
                        {
                            environment = { name = "dev" },
                            state = "DEPLOYED",
                        },
                    },
                },
            },
        }
    )
end)
