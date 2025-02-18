# Integrartion test framework

This package contains a custom integration framework that let's you test
the entire system using a folder with test files. There's support to test
the system using:

- REST
- GraphQL
- SQL
- PubSub

After each file is run, the reconciliation loop is run to ensure that the
system is in a stable state before running the next test.
To disable this, set the `reconcile` flag to `false` in the config on each environment.

## How to use it

Within the `testdata` folder you can create a folder with the name of the
test you want to run. Within that folder you can create one or more files
defining your test cases as described below.

### Templating

The entire test file is run through a templating engine before being run.
This means that you can use the following variables in your test files:

| Variable                          | Description                                                     |
| --------------------------------- | --------------------------------------------------------------- |
| `{{ .Tenant }}`                   | A map of tenants defined in the config file.                    |
| `{{ .Tenant.<name> }}`            | A specific tenant defined in the config file.                   |
| `{{ .Tenant.<name>.Env }}`        | A map of environments defined on the tenant in the config file. |
| `{{ .Tenant.<name>.Env.<name> }}` | A specific environment.                                         |

Both `.Tenant.<name>` and `.Tenant<name>.Env.<name>` contains information about that object, e.g.:

| Variable | Description            |
| -------- | ---------------------- |
| `.ID`    | The ID of the object   |
| `.Name`  | The name of the object |

If you store fields from the response of a test, using `STORE <key>=<response key>`, you can access them using `{{ .<key> }}`.

## Configuration

You can configure the test framework using a file called `00_config.yaml`.

To see the full list of configuration options, see the `Config` struct in
[`testrunner_config.go`](./testrunner_config.go)

### Example

```yaml
# List of tenants within the test
tenants:
  - name: tenant23
    ci: true # This tenant has the CI flag set to true
    envs:
      # List of environments within the tenant
      - kind: management # One of management, tenant, onprem, legacy
        name: management
        ci: true # This environment has the CI flag set to true
        naisd: # Configuration for the naisd component
          enabled: true # Enable naisd
          successfullMessages: 100 # How many successfull messages to return until starting to return errors
        reconcile: true # Set the reconcile flag to true

      - kind: tenant
        name: nonci
        ci: false
        naisd:
          enabled: true
          successfullMessages: 100
        reconcile: true
```

## REST

To test a REST endpoint, create a file with the extension `.rest.test`.

The overall structure of the file is as follows:

```
[METHOD] [PATH]

[BODY]

RETURNS

OPTION [OPTIONS]

ENDOPTS

[EXPECTED RESPONSE]

STORE [NAME]=[JSONPATH]
```

### Example

```
POST /github/rollout

{
  "chart": "oci://clamav",
  "version": "0.1.0-feature"
}

RETURNS

OPTION responseCode = 201
OPTION id=IGNORE

ENDOPTS

{
  "envNotAvailable": ["tenant"]
}

STORE rollout_id=id
```

### Options

| Option         | Description                                    |
| -------------- | ---------------------------------------------- |
| `responseCode` | The expected response code. Defaults to `200`. |
| `key=IGNORE`   | Ignore the `key` when comparing the response.  |

### GraphQL

To test a GraphQL endpoint, create a file with the extension `.gql.test`.

The overall structure of the file is as follows:

```
[QUERY]

RETURNS

OPTION [OPTIONS]

ENDOPTS

[EXPECTED RESPONSE]

STORE [NAME]=[JSONPATH]
```

### Example

```
query {
  applications {
    id
    name
    environments {
      name
    }
  }
}

RETURNS

OPTION id=IGNORE

{
  "data": {
    "applications": {
      "name": "my-app",
      "environments": [
        {
          "name": "t1"
        },
        {
          "name": "t2"
        }
      ]
    }
  }
}

STORE app_id=id
```

### Options

| Option       | Description                                   |
| ------------ | --------------------------------------------- |
| `key=IGNORE` | Ignore the `key` when comparing the response. |

## SQL

To test a SQL query, create a file with the extension `.sql.test`.

The overall structure of the file is as follows:

```
[QUERY]

RETURNS

[EXPECTED RESPONSE AS JSON]
```

### Example

```
SELECT count(1)::float
FROM tenants;

RETURNS

[{"count": 1}]
```

## PubSub

To test a PubSub message, create a file with the extension `.pubsub.test`.

There's two types of cases with PubSub, one for sending and one for receiving.

### Sending

Currently untested, but might work.

```
{DATA}

RETURNS

OPTION topic=[TOPIC]

ENDOPTS
```

### Receiving

```
RETURNS

OPTION topic=[TOPIC]
OPTION id=IGNORE

ENDOPTS

{DATA}

STORE id=id
```

### Options

| Option       | Description                                   |
| ------------ | --------------------------------------------- |
| `topic`      | The topic to listen or send to. **Required**  |
| `key=IGNORE` | Ignore the `key` when comparing the response. |
