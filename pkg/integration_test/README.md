# Integrartion test framework

This package contains a custom integration framework that let's you test
the entire system using a folder with test files. There's support to test
the system using:

- REST
- GraphQL
- SQL
- PubSub

## How to use it

Within the `testdata` folder you can create a folder with the name of the
test you want to run. Within that folder you can create one or more files
defining your test cases as described below.

## Configuration

You can configure the test framework using a file called `00_config.json`.

The following options are available:

| Option                       | Type   | Description                                                                                                                                 |
| ---------------------------- | ------ | ------------------------------------------------------------------------------------------------------------------------------------------- |
| `no_naisd`                   | `bool` | If set to `true`, naisd will not run for this test case.                                                                                    |
| `no_tenants`                 | `bool` | If set to `true`, No seed data for tenants and environments will be created.                                                                |
| `ci`                         | `bool` | If set to `true`, the seeded tenant and environment named `ci` will be marked as `ci`.                                                      |
| `reconcile`                  | `bool` | If set to `true`, the reconciler will run for this test case.                                                                               |
| `naisd_successfull_messages` | `int`  | If set, the number of successfull messages naisd will send to the reconciler before sending failures. Will allways be successfull if unset. |

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
