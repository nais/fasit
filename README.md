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
```

## Testing

Run all tests in the project with `make integration-test`.

### Integration tests

This package contains a custom integration framework that let's you test
the entire system using a folder with test files.

Read more about the integration test framework in [`pkg/integration_test/README.md`](./pkg/integration_test/README.md).

# Releasing

## Fasit

Fasit is released whenever a new push to main is done.

The action will build a new image, push it to GAR and then deploy it using helm.

## naisd

naisd is released by pushing a tag in the format `naisd-<version>`.
This will build a new image, push it to GAR and then roll it out using Fasit.
