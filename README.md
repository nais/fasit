# Fasit

Fasit manages feature deployments across environments for NAIS tenants.
It consists of two components:

- **fasit** — HTTP/gRPC server with a web UI for managing environments, features, deployments, and labels
- **naisd** — per-environment agent that reconciles feature state using Helm

## Local development

```sh
# Install tools
mise install

# Configure environment
cp .env.example .env

# Start postgres + pubsub emulator and seed the database
mise run setup

# Run fasit with auto-reload on changes
mise run dev

# Or without auto-reload
mise run fasit
```

### Running naisd locally

```sh
# Run naisd for all seeded environments in parallel.
# test-partner/prod uses --mock-failing and test-partner/staging is left
# without naisd, giving a mix of DEPLOYED, FAILED, and PENDING states.
mise run naisd-all
```

Single-environment alternatives:

- `mise run naisd` — test-partner/dev
- `mise run naisd-failing` — test-partner/dev with mocked helm failures
- `mise run naisd-management` — test-partner/management
- `mise run naisd-management-failing` — test-partner/management with mocked helm failures

## Features

Features are OCI Helm charts with a `Feature.yaml` alongside the `Chart.yaml`.
See [`docs/feature.md`](./docs/feature.md) for the spec.

### JSON schema

Enable autocompletion/validation with:
`https://storage.googleapis.com/fasit-jsonschema/feature.json`

See https://docs.nais.io/appendix/json-schema/ for IDE setup.

## Testing

```sh
mise run test          # unit + integration tests
mise run test:unit     # unit tests only
mise run test:integration  # integration tests (requires testcontainers)
```

## Static checks

```sh
mise run check   # lint, fmt, vet
```

## Releasing

### Fasit

Pushed to main → image built, pushed to GAR, deployed via Helm.
Chart changes require manually updating the workflow.

### naisd

Tag `naisd-<version>` → image built, pushed to GAR, rolled out via Fasit.

## Access production postgres

```sh
# Retrieve password (requires nais-io tenant access)
kubectl --context nais-io -n nais-system get secrets fasit-backend-db -o json \
  | jq -r '.data.FASIT_DBCONN_STRING' | base64 -d | awk -F '=' '{print $6}'

# Connect
gcloud sql connect fasit --project nais-io --user fasit --database fasit
```
