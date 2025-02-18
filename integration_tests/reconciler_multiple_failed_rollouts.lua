local tenantID = Helper.CreateTenant("tenant23", true)
local managementID = Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "testing", "tenant", false, true)
local nonciID = Helper.CreateEnvironment(tenantID, "nonci", "tenant")

-- Seed feature data
Helper.SQLExec [[
INSERT INTO feature_data (
  name,
  version,
  chart,
  description,
  source,
  kinds,
  dependencies,
  values,
  default_values
) VALUES
  ('feature1', '1', 'oci://chart', '', '', '{management,tenant}'::environment_kind[], 'null', '{}', '{}'),
  ('feature1', '2', 'oci://chart', '', '', '{management,tenant}'::environment_kind[], 'null', '{}', '{}'),
  ('feature1', '3', 'oci://chart', '', '', '{management,tenant}'::environment_kind[], 'null', '{}', '{}'),
  ('feature2', '1', 'oci://chart', '', '', '{management,tenant}'::environment_kind[], 'null', '{}', '{}')
;
]]

Helper.SQLExec [[
INSERT INTO features (
  name,
  version
) VALUES
  ('feature1', '1')
;
]]

Helper.SQLExec [[
INSERT INTO rollouts (
  feature_name,
  version,
  status,
  created
) VALUES
  ('feature1','1', 'deployed', now() - interval '2 days'),
  ('feature1','2', 'failed', now() - interval '1 day'),
  ('feature1','3', 'failed', now())
;
]]

Test.gql("enable features", function(t)
	t.query(string.format([[
		mutation {
			testing: featureStateSave(envID: "%s", enabled: true, feature: "feature1") {
				enabled
			}

			mgmt: featureStateSave(envID: "%s", enabled: true, feature: "feature1") {
				enabled
			}

			nonci: featureStateSave(envID: "%s", enabled: true, feature: "feature1") {
				enabled
			}
		}
	]], testingID, managementID, nonciID))

	t.check {
		data = {
			testing = {
				enabled = true,
			},
			mgmt = {
				enabled = true,
			},
			nonci = {
				enabled = true,
			},
		},
	}
end)


Test.sql("validate deploy instructions", function(t)
	Helper.Reconcile()
	t.query [[
		SELECT
			(SELECT name FROM environments WHERE environments.id = environment_id) AS environment,
			feature_name,
			feature_version,
			status
		FROM deploy_instructions
		ORDER BY environment, feature_name, feature_version
	]]

	t.check {
		{
			environment = "management",
			feature_name = "feature1",
			feature_version = "1",
			status = "deployed",
		},
		{
			environment = "nonci",
			feature_name = "feature1",
			feature_version = "1",
			status = "deployed",
		},
		{
			environment = "testing",
			feature_name = "feature1",
			feature_version = "1",
			status = "deployed",
		},
	}
end)
