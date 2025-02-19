local tenantID = Helper.CreateTenant("tenant23", true)
local managementID = Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "testing", "tenant", false, true)
local nonciID = Helper.CreateEnvironment(tenantID, "nonci", "tenant")

-- Seed feature data
Helper.SQLExec([[
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
  ('feature1', '3', 'oci://chart', '', '', '{management,tenant}'::environment_kind[], 'null', '{}', '{}')
;
]])

Helper.SQLExec([[
	INSERT INTO rollouts (
  feature_name,
  version,
  status,
  created
) VALUES
  ('feature1','1', 'failed', now() - interval '2 days'),
  ('feature1','2', 'failed', now() - interval '1 day'),
  ('feature1','3', 'failed', now())
;
]])

Helper.SQLExec([[
	INSERT INTO feature_states (
	environment_id,
	feature,
	enabled,
	enabled_at
) VALUES
(
	$1,
	'feature1',
	true,
	now()
),
(
	$2,
	'feature1',
	true,
	now()
),
(
	$3,
	'feature1',
	true,
	now()
)
]], testingID, managementID, nonciID)

Helper.Reconcile();

Test.sql("Validate deploy instructions", function(t)
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
			feature_version = "3",
			status = "deployed",
		},
		{
			environment = "testing",
			feature_name = "feature1",
			feature_version = "3",
			status = "deployed",
		},
	}
end)

Test.gql("list features", function(t)
	t.query [[
		query {
			tenants {
				environments {
					name
					features {
						name
						version
					}
				}
			}
		}
	]]

	t.check {
		data = {
			tenants = {
				{
					environments = {
						{
							name = "management",
							features = {
								{
									name = "feature1",
									version = "3",
								},
							},
						},
						{
							name = "nonci",
							features = {},
						},
						{
							name = "testing",
							features = {
								{
									name = "feature1",
									version = "3",
								},
							},
						},
					},
				},
			},
		},
	}
end)
