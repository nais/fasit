# fasit

## Features

Fasit contains a set of features that can be enabled for a tenant.
Each feature is defined as an OCI chart with a file called `Feature.yaml` besides the `Chart.yaml` file.

Documentation for how to define a feature can be found in [`docs/feature.md`](./docs/feature.md).

### JSON schema

To enable autocompletion and validation you can add the following json schema to your IDE:
`https://storage.googleapis.com/fasit-jsonschema/feature.json`

Follow the guide on https://docs.nais.io/appendix/json-schema/ on how to add a json schema.

## local dev setup

```
docker-compose up

# Run backend
make local

# Build config
make setup

# Run naisd
make local-naisd

# Install tools
mise install
```

### Test local rollout

```bash
./hack/local_rollout.sh <chart> <version>
```

For the rollout to progress, you need to have naisd running, either with `make local-naisd` or `make local-naisd-failing`.

You will also need to update `test-partner` `dev` to be a CI environment.

## Testing

Run all tests in the project with `make test`.

### Integration tests

We are using [tester](https://github.com/nais/tester) for integration tests.
These tests are written in Lua and can be found in the `integration_tests` directory.

A spec file is generated to support auto-completion using the Lua language server.

When running `make test` the integration tests will be run as part of the test suite.

To run the integration tests in watch mode, run `make integration_test_ui`.
This will start a web server on `localhost:9876` where you can see the test results.
They will be re-run every time you save a `.lua` file.

# Releasing

## Fasit

Fasit is released whenever a new push to main is done.

The action will build a new image, push it to GAR and then deploy it using helm.

## naisd

naisd is released by pushing a tag in the format `naisd-<version>`.
This will build a new image, push it to GAR and then roll it out using Fasit.

## Access production postgres

Retrieve password (require connecting to nais-io tenant):

```
kubectl --context nais-io -n nais-system get secrets fasit-backend -o json | jq -r '.data.FASIT_DBCONN_STRING' | base64 -d | awk -F '=' '{print $6}'
```

Connect to postgres:

```
gcloud sql connect fasit --project nais-io --user fasit
```
