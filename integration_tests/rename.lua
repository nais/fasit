local tenantID = Helper.CreateTenant("tenant1", true)
local managementID = Helper.CreateEnvironment(tenantID, "management", "management", false, true)
local testingID = Helper.CreateEnvironment(tenantID, "e1", "tenant", false, true)
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


-- Even tough nonci doesn't have the feature available until it has rolled out, there's nothing
-- stopping us from configuring and enabling it.
local keys = { "myValue", "nested.key", "nested.computed.key" }

Test.gql("enable feature", function(t)
	local mutations = {}
	for i, key in ipairs(keys) do
		table.insert(mutations, i, string.gsub([[
			tc_%index: configurationCreate(configuration: {environmentID: "%testingID", feature:"rename", key: "%key", value: "%value"}) {
				source
			}
			mc_%index: configurationCreate(configuration: {environmentID: "%managementID", feature:"rename", key: "%key", value: "%value"}) {
				source
			}
			nonci_%index: configurationCreate(configuration: {environmentID: "%nonciID", feature:"rename", key: "%key", value: "%value"}) {
				source
			}
			glob_%index: configurationCreate(configuration: {feature:"rename", key: "%key", value: "%value"}) {
				source
			}
		]], "%%%w+", {
			["%index"] = string.format("%d", i),
			["%testingID"] = testingID,
			["%managementID"] = managementID,
			["%nonciID"] = nonciID,
			["%key"] = key,
			["%value"] = key .. "_value"
		}))
	end

	local mutation = table.concat(mutations, "\n")

	t.query(string.format([[
		mutation {
			%s
			featureStateSave(envID: "%s", enabled: true, feature: "rename") {
				enabled
			}
			mgmt: featureStateSave(envID: "%s", enabled: true, feature: "rename") {
				enabled
			}
			nonci: featureStateSave(envID: "%s", enabled: true, feature: "rename") {
				enabled
			}
		}
		]], mutation, testingID, managementID, nonciID))


	local checkData = {
		data = {}
	}

	for i, key in ipairs(keys) do
		checkData["data"]["tc_" .. i] = {
			source = "ENV"
		}
		checkData["data"]["mc_" .. i] = {
			source = "ENV"
		}
		checkData["data"]["nonci_" .. i] = {
			source = "ENV"
		}
		checkData["data"]["glob_" .. i] = {
			source = "GLOBAL"
		}
	end

	checkData["data"]["featureStateSave"] = {
		enabled = true
	}
	checkData["data"]["mgmt"] = {
		enabled = true
	}
	checkData["data"]["nonci"] = {
		enabled = true
	}

	t.check(checkData)
end)

Test.gql("validate configs", function(t)
	t.query [[
	 {
		tenants {
			name
			environments {
				name
				feature(name:"rename") {
					configuration {
						configuration {
							content
							value {
								key
							}
						}
					}
				}
			}
		}
	}
	]]

	t.check {
		data = {
			tenants = {
				{
					name = "tenant1",
					environments = {
						{
							name = "management",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "e1",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "nonci",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "nested.key"
											}
										},
									}
								}
							}
						}
					}
				}
			}
		}
	}
end)

Test.gql("validate state", function(t)
	Helper.Reconcile() -- Reconcile CI environments, rollout only matches CI
	Helper.Reconcile() -- Reconcile all environments

	t.query([[
		{
			tenants {
				name
				environments {
					name
					feature(name: "rename") {
						status {
							status
							version
						}
					}
				}
			}
		}
	]])

	t.check
	{
		data = {
			tenants = {
				{
					name = "tenant1",
					environments = {
						{
							name = "management",
							feature = {
								status = {
									status = "DEPLOYED",
									version = "1.0"
								}
							}
						},
						{
							name = "e1",
							feature = {
								status = {
									status = "DEPLOYED",
									version = "1.0"
								}
							}
						},
						{
							name = "nonci",
							feature = {
								status = {
									status = "DEPLOYED",
									version = "1.0"
								}
							}
						},
					}
				}
			}
		}
	}
end)

-- Disable reconcilers
Helper.SQLExec("UPDATE environments SET reconcile = false;")

Test.rest("create rollout v2", function(t)
	t.send("POST", "/github/rollout", [[
	{
		"chart": "oci://rename",
		"version": "2.0"
	}
]])

	t.check(201, {
		id = Save("rolloutv2_id"),
		envNotAvailable = {}
	})
end)


Test.gql("check config", function(t)
	t.query [[
	 {
		tenants {
			name
			environments {
				name
				feature(name:"rename") {
					configuration {
						configuration {
							content
							value {
								key
							}
						}
					}
				}
			}
		}
	}
]]

	t.check {
		data = {
			tenants = {
				{
					name = "tenant1",
					environments = {
						{
							name = "management",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "e1",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "nonci",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "nested.key"
											}
										},
									}
								}
							}
						}
					}
				}
			}
		}
	}
end)

-- Re-enable reconcilers
Helper.SQLExec("UPDATE environments SET reconcile = true;")

Test.sql("validate rollout", function(t)
	Helper.Reconcile()

	t.queryRow([[
		SELECT status, version FROM rollouts WHERE id = $1
	]], State.rolloutv2_id)

	t.check({
		status = "deployed",
		version = "2.0"
	})
end)

Test.gql("check config after rollout", function(t)
	t.query [[
	 {
		tenants {
			name
			environments {
				name
				feature(name:"rename") {
					configuration {
						configuration {
							content
							value {
								key
							}
						}
					}
				}
			}
		}
	}
]]

	t.check {
		data = {
			tenants = {
				{
					name = "tenant1",
					environments = {
						{
							name = "management",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "e1",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "nonci",
							feature = {
								configuration = {
									configuration = {
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						}
					}
				}
			}
		}
	}
end)

-- Create some configuration values that shouldn't be renamed, because the target key already exists from the earlier steps
Helper.SQLExec("INSERT INTO configurations_global (feature, key, value) VALUES ('rename', 'myValue', '\"oldVal\"');")
Helper.SQLExec(
	string.format(
		"INSERT INTO configurations_environment (feature, key, value, environment_id) VALUES ('rename', 'nested.key', '\"oldVal\"', '%s');",
		managementID
	)
)

Test.gql("check config with non-renamed configs", function(t)
	t.query [[
		{
			tenants {
				name
				environments {
					name
					feature(name:"rename") {
						configuration {
							configuration {
								content
								value {
									key
								}
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			tenants = {
				{
					name = "tenant1",
					environments = {
						{
							name = "management",
							feature = {
								configuration = {
									configuration = {
										{
											content = "oldVal",
											value = {
												key = "myValue"
											}
										},
										{
											content = "oldVal",
											value = {
												key = "nested.key"
											}
										},
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "e1",
							feature = {
								configuration = {
									configuration = {
										{
											content = "oldVal",
											value = {
												key = "myValue"
											}
										},
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "nonci",
							feature = {
								configuration = {
									configuration = {
										{
											content = "oldVal",
											value = {
												key = "myValue"
											}
										},
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
end)

Test.rest("create rollout v3", function(t)
	t.send("POST", "/github/rollout", [[
	{
		"chart": "oci://rename",
		"version": "3.0"
	}
]])

	t.check(201, {
		id = Save("rollout_id"),
		envNotAvailable = {}
	})
end)

Test.gql("check config after v3", function(t)
	Helper.Reconcile()

	t.query [[
		{
			tenants {
				name
				environments {
					name
					feature(name:"rename") {
						configuration {
							configuration {
								content
								value {
									key
								}
							}
						}
					}
				}
			}
		}
	]]

	t.check {
		data = {
			tenants = {
				{
					name = "tenant1",
					environments = {
						{
							name = "management",
							feature = {
								configuration = {
									configuration = {
										{
											content = "oldVal",
											value = {
												key = "myValue"
											}
										},
										{
											content = "oldVal",
											value = {
												key = "nested.key"
											}
										},
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "e1",
							feature = {
								configuration = {
									configuration = {
										{
											content = "oldVal",
											value = {
												key = "myValue"
											}
										},
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									}
								}
							}
						},
						{
							name = "nonci",
							feature = {
								configuration = {
									configuration = {
										{
											content = "oldVal",
											value = {
												key = "myValue"
											}
										},
										{
											content = "myValue_value",
											value = {
												key = "rename.myValue"
											}
										},
										{
											content = "nested.computed.key_value",
											value = {
												key = "rename.nested.computed.key"
											}
										},
										{
											content = "nested.key_value",
											value = {
												key = "rename.nested.key"
											}
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
end)
