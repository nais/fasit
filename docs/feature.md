# Feature V2

The feature V2 spec is the current version used in Fasit.
It is build as an extension to [Helm OCI charts](https://helm.sh/docs/topics/registries/).

## Create a feature

Start by creating a new chart:

```bash
helm create my-feature
```

This will create a directory called `my-feature`.
Within this directory you'll define the resources that should be created when the feature is enabled.

There's a few fields used by Fasit that you need to add to the `Chart.yaml` file:

```yaml
# This is the name of the feature, as well as the name of the chart.
name: my-feature

# Description of what the feature does.
description: My feature creates some test resources

# Sources should be set with at least one source, pointing to the git repository and path to the chart.
sources:
  - github.com/nais/my-feature/tree/main/chart
```

To make the chart into a feature, you must create a `Feature.yaml` file in the same directory as the `Chart.yaml` file.

Minimal example:

```yaml
# environmentKinds is required, and defines which environments the feature can be enabled in.
environmentKinds:
  - tenant
```

Complete example:

```yaml
environmentKinds:
  - management
  - tenant
  - onprem
  - legacy
# list of values that will be provided by Fasit, either as a computed value or manually entered (config).
values:
  # The key is the key used in the values.yaml file.
  # For nested keys, use dot notation. If the values.yaml file defines:
  #   image:
  #     repository: my-image
  # Then the key would be `image.repository`.
  #
  # If the key in the values.yaml file inclues a dot, you can escape it with a backslash.
  #   image.repository: my.image
  # Then the key would be `image\.repository`.
  image.tag:
    # displayName is the name of the field shown in the UI.
    displayName: Image tag to deploy
    # description can describe the field in more detail.
    description: The tag of the image to deploy
    # config defines user input
    config:
      # type of input, can be string, int, bool or string_array.
      type: string
  someSecret:
    config:
      type: string
      # secret defines that the value should be stored as a secret in Fasit.
      secret: true
      # required defines if the field is required or not.
      required: true
  cluster:
    displayName: Cluster name
    # computed defines a value that will be computed by Fasit. Usually this utilizes values stored in Fasit by Terraform.
    computed:
      # template is a go template which will then be parsed as YAML. Make sure that the output type matches the type you want.
      template: "{{ .Env.name }}"
  mixed:
    displayName: Both config and computed
    # If both config and computed is defined, the computed value will be used unless
    # a value is set for the environment or globally in Fasit.
    config:
      type: string
    computed:
      template: "{{ .Env.value | quote }}"
```

> Values should only be defined in the `Feature.yaml` file if they either are helpful for us to debug etc, or they differ between environments/tenants.

## Template funcs

The following template funcs are available:

| Name                             | Description                                                                                                                                                                                   |
| -------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `b64enc(string)`                 | Base64 encodes the given string.                                                                                                                                                              |
| `eachOf(slice, key)`             | `eachOf` returns a list of values by iterating over the given slice and getting the value using the given key. _The slice must be a slice of maps or structs._                                |
| `environmentsAsMap(keys, []map)` | Mainly for use with `.Envs`, create a map with the name of the environment as key and a map with the value of the `keys` field as value.                                                      |
| `filter(key, value, slice)`      | Filters the slice by the given key and value.                                                                                                                                                 |
| `fromJSON(value)`                | Converts the value from JSON.                                                                                                                                                                 |
| `join(sep, slice)`               | Joins the elements of the slice with the given separator.                                                                                                                                     |
| `mapJoin(sep, map)`              | Join each key and value in the map with the separator.                                                                                                                                        |
| `mapOf(name, value, []map)`      | For each element in the array, create a map with the value of the `name` field as key and the value of the `value` field as value.                                                            |
| `prefixedValues(map, prefix)`    | Return a list of values from the map where the key has the given prefix.                                                                                                                      |
| `quote`                          | Quotes the string.                                                                                                                                                                            |
| `replace(string, old, new)`      | Replaces all occurrences of `old` with `new` in the given string.                                                                                                                             |
| `subdomain(root, subdomain) `    | Return a subdomain for the given context. Will create a `[subdomain].tenant.cloud.nais.io` if the target environment is `management`, and `[subdomain].environment.tenant.nais.io` otherwise. |
| `toJSON(value)`                  | Converts the value to JSON.                                                                                                                                                                   |
| `toYAML(value)`                  | Converts the value to YAML.                                                                                                                                                                   |

We also includes all functions from [Sprig](https://masterminds.github.io/sprig/).

> [!IMPORTANT]
> If any function from Sprig has the same name as one of our functions, our function will be used.
