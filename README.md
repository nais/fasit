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

```sh
# Install tools
mise install

# Configure environment
cp .env.example .env

# Start postgres + pubsub emulator and seed the local database with
# tenants, environments, features, and deployments.
mise run setup

# Run fasit (in its own terminal)
mise run fasit

# Run naisd for every seeded tenant/env in parallel (in its own terminal).
# test-partner/prod is configured with --mock-failing and test-partner/staging
# is intentionally left out (no naisd) so the local UI ends up with a mix of
# DEPLOYED, FAILED, and PENDING features.
mise run naisd-all
```

`mise run naisd-all` is a wrapper around the lower-level `naisd-run` helper.
If you only need a single env (for example to debug `test-partner/dev`),
you can also run any of:

- `mise run naisd` — test-partner/dev
- `mise run naisd-failing` — test-partner/dev with mocked helm failures
- `mise run naisd-management` — test-partner/management
- `mise run naisd-management-failing` — test-partner/management with mocked helm failures

### Test local rollout

```bash
./hack/local_rollout.sh <chart> <version>
```

For the rollout to progress, you need to have naisd running, either with `mise run naisd`.

You will also need to update `test-partner` `dev` to be a CI environment.

## Testing

Run all tests in the project with `mise run test`.

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

Changes in the chart require manually updating the Helm-command in the workflow.

## naisd

naisd is released by pushing a tag in the format `naisd-<version>`.
This will build a new image, push it to GAR and then roll it out using Fasit.

## Access production postgres

Retrieve password (require connecting to nais-io tenant):

```
kubectl --context nais-io -n nais-system get secrets fasit-backend-db -o json | jq -r '.data.FASIT_DBCONN_STRING' | base64 -d | awk -F '=' '{print $6}'
```

Connect to postgres:

```
gcloud sql connect fasit --project nais-io --user fasit --database fasit
```
