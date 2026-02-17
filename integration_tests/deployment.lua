local tenantID = Helper.CreateTenant("tenant1", false)
Helper.CreateEnvironment(tenantID, "management", "management", false, true, { kind = "management" })
Helper.CreateEnvironment(tenantID, "dev", "tenant", false, false, { kind = "tenant" })
Helper.CreateEnvironment(tenantID, "prod", "tenant", false, false, { kind = "tenant" })
Helper.CreateEnvironment(tenantID, "nonci", "tenant", false, false, { kind = "tenant" })

Test.rest("create deployment", function(t)
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

Test.rest("create second deployment", function(t)
	t.send("POST", "/github/deployment", [[
		{
			"chart": "oci://allenvs",
			"version": "1.0.0",
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

Test.gql("list deployments", function(t)
	t.query [[
		{
			deployments {
				id
				feature {
					name
					version
				}
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
				feature {
					name
					version
				}
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
						feature = {
							name = "clamav",
							version = "0.1.0-feature",
						},
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

Helper.Reconcile()

Test.gql("list deployments by feature with status", function(t)
	t.query [[
		{
			deployments (feature: "clamav") {
				feature {
					name
					version
				}
				statuses {
					environment {
						name
						kind
						labels {
							key
							value
						}
					}
					state
					message
				}
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
						feature = {
							name = "clamav",
							version = "0.1.0-feature",
						},
						statuses = {
							{
								environment = {
									kind = "TENANT",
									labels = {
										{
											key = "kind",
											value = "tenant",
										},
									},
									name = "prod",
								},
								message = "received status from naisd.",
								state = "DEPLOYED",
							},
							{
								environment = {
									kind = "TENANT",
									labels = {
										{
											key = "kind",
											value = "tenant",
										},
									},
									name = "nonci",
								},
								message = "received status from naisd.",
								state = "DEPLOYED",
							},
							{
								environment = {
									kind = "TENANT",
									labels = {
										{
											key = "kind",
											value = "tenant",
										},
									},
									name = "dev",
								},
								message = "received status from naisd.",
								state = "DEPLOYED",
							},
						},
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

Test.rest("create global deployment", function(t)
	t.send("POST", "/github/deployment", [[
		{
			"chart": "oci://global",
			"version": "1.0.0",
			"target": {},
			"ci": {"wait": true}
		}
	]])

	t.check(201, {
		id = Save("globalDeploymentId"),
	})
end)

Helper.Reconcile()


Test.gql("get global deployment", function(t)
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
						kind
						labels {
							key
							value
						}
					}
					state
					message
				}
				target {
				    key
				    value
				}
			}
		}
	]], State.globalDeploymentId))

	t.check(
		{
			data = {
				deployment = {
					feature = {
						name = "global",
						version = "1.0.0",
					},
					statuses = {
						{
							environment = {
								kind = "TENANT",
								labels = {
									{
										key = "kind",
										value = "tenant",
									},
								},
								name = "prod",
							},
							message = "received status from naisd.",
							state = "DEPLOYED",
						},
						{
							environment = {
								kind = "TENANT",
								labels = {
									{
										key = "kind",
										value = "tenant",
									},
								},
								name = "nonci",
							},
							message = "received status from naisd.",
							state = "DEPLOYED",
						},
						{
							environment = {
								kind = "MANAGEMENT",
								labels = {
									{
										key = "kind",
										value = "management",
									},
								},
								name = "management",
							},
							message = "received status from naisd.",
							state = "DEPLOYED",
						},
						{
							environment = {
								kind = "TENANT",
								labels = {
									{
										key = "kind",
										value = "tenant",
									},
								},
								name = "dev",
							},
							message = "received status from naisd.",
							state = "DEPLOYED",
						},
					},
					target = {},
				},
			},
		}
	)
end)
