# Fasit Feature

A Fasit feature is defined as a YAML-file with information about which chart to deploy and available configurations.

The name of the file is the name Fasit will use for the feature.

## Required metadata

### `chart` (String)

The `chart` field defines the name of the chart to deploy.

You can choose to use a chart based on an OCI image by using the full path to the image, e.g. `oci://europe-north1-docker.pkg.dev/nais-io/nais/loadbalancer`.

If you want to use a chart from a registry, you define only the name of the chart, e.g. `loadbalancer`.
In this case, you will have to specify the Helm repo in the `repo` field.

### `version` (String)

The `version` field defines the version of the chart to deploy.

### `source` (String)

The `source` field defines the source of the chart to deploy.
This is the code source, where you should be able to find the chart contents.

### `environmentKinds` ([]String)

The `environmentKinds` field defines the kind of environments the feature is available in.
Supported kinds are `management` and `tenant`.

## Optional metadata

### `repo` (String)

The `repo` field defines the Helm repo to use for the chart.
This is required unless you use an OCI image.

### `timeout` (String)

Default `5m`.

The `timeout` field defines the maximum time before a rollout is cancelled.
The format is `0h0m0s`, where one or more of the parts can be omitted.
E.g. `15m` for 15 minutes or `1h30m` for 1 hour and 30 minutes.

### `dependsOn` ([]String)

The `dependsOn` field defines the features that must be deployed before this feature is deployed.

### `config` (Object)

The configuration object defines the configuration for the feature.
Each key is mapped to a value in Helm.

The key is split on `.` and nested objects are created.
If a key needs `.` in it, it should be escaped with `\`.

The value of each key is an object with the following fields:

| Field         | Type   | Description                                                                    | Required |
| ------------- | ------ | ------------------------------------------------------------------------------ | -------- |
| `type`        | String | The type of the value.<br>Supports `string`, `string_array`, `bool` and `int`. | Yes      |
| `displayName` | String | Display name used by the frontend                                              |          |
| `required`    | Bool   | Whether the value is required or for deployment or not.                        |          |
| `secret`      | Bool   | Whether the value is a secret or not.                                          |          |

### `mapping` (Object)

The mapping object defines the mapping for the feature.
Each key is mapped to a value in Helm.

The key is split on `.` and nested objects are created.
If a key needs `.` in it, it should be escaped with `\`.

The value of each key is an object with the following fields:

| Field         | Type                       | Description                                                                              | Required                |
| ------------- | -------------------------- | ---------------------------------------------------------------------------------------- | ----------------------- |
| `template`    | String                     | The string will be processed by Go text/template. The result must be valid yaml          | Yes (or `value`)        |
| `value`       | String or array of strings | The string will be processed by Go template, and the result will be treated as a string. | Yes (One or `template`) |
| `displayName` | String                     | Display name used by the frontend                                                        |                         |

The `template` and `value` field has access to the following data:

```go
type MappingValues struct {
	// Kind is the kind of environment the feature is deployed to.
	Kind model.EnvironmentKind
	// Tenant is information about the tenant that owns the cluster the feature is deployed to.
	Tenant struct {
		Name string
	}
	// Management is information about the management cluster for the tenant.
	Management map[string]any
	// Env contains information about the cluster the feature is deployed to.
	Env map[string]any
	// Envs contains information about all clusters the tenant has access to.
	Envs []map[string]any
}
```

The only value that is certain to be available in `Management`, `Env` and for each environment in `Envs` is `name`, which contains the name of the environment.

All other values are created by Terraform and can be viewed in the Fasit UI.
Go to `https://fasit.nais.io`, select a tenant and choose which environment you want to view the values for.
On the bottom of the overview page you will see a list of all the values.

You can also check [nais-terraform-modules](https://github.com/nais/nais-terraform-modules),
By convention, all values are defined in `fasit.tf` files in both `module-tenant` and `module-management` modules.

#### Functions

| Function                                                   | Description                                                                                                                |
|------------------------------------------------------------| -------------------------------------------------------------------------------------------------------------------------- |
| `mapOf(ikey, ivalue string, envs []map[string]any)`        | Creates a new map with key being the value of `ikey` and value being the value of `ivalue` from each env in `envs`.        |
| `replace(s, old, new string)`                              | Replaces all occurrences of `old` with `new` in `s`.                                                                       |
| `mapJoin(sep string, m any)`                               | For each key-value pair in `m`, the key and value is joined with `sep` and all returned as a slice                         |
| `prefixedValues(m any, prefix string)`                     | Return a slice with values from `m` where the key has the prefix `prefix`                                                  |
| `subdomain(m *MappingValues, subdomain string)`            | Returns the subdomain for the environment. It will return a subdomain for the environment it's installed in.               |
| `environmentsAsMap(keyList string, data []map[string]any)` | Works on `MappingVlaues.Envs` and will return a map with the environment name as key, and the value of `keyList` as value. |
| `eachOf(m any, key string)`                                | Returns a list of values by iterating over `m` (slice or array) and getting the value using `key`.                         |
| `toJSON(v any)`                                            | Returns the JSON representation of `v`.                                                                                    |
| `fromJSON(s string)`                                       | Decode JSON and return `map[string]any{}`.                                                                                 |
| `toYAML(v any)`                                            | Returns the YAML representation of `v`.                                                                                    |
| `join(sep string, v any)`                                  | Joins the values in `v` with `sep`, where `v` is any slice                                                                 |
| `filter(key string, v any)`                                | Filters the values in `v` by the key `key`. `v` must be a slice                                                            |

# JSON Schema and testing

A JSON Schema is generated from the Go structs defining the feature (`make generate-feature-schema`).
The schema is used when running the tests in `features_test.go`.

You can also use the schema in your editor, e.g. in VSCode, create a `.vscode` directory and put the following in `settings.json`.

```json
{
  "yaml.schemas": {
    "./schema/jsonschema/feature.json": "features/*.yaml"
  }
}
```
