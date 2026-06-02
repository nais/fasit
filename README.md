# Fasit

Fasit is a multi-tenant feature management platform. It deploys OCI Helm charts — called **features** — across environments, using label-based targeting and automatic reconciliation.

## How it works

```
  Feature author
       │
       ├─── push chart ───────► OCI Registry
       │                              ▲
       └─── create assignment ──┐     │
                                ▼     │
     ┌────── Fasit ─────────────────────────────┐
     │                                          │
     │  Desired state        Reconciler         │
     │  ┌─────────────┐         │               │
     │  │ assignment  │────► compare ◄──────┐   │
     │  │ config      │         │           │   │
     │  │ env values  │    deploy instr.    │   │
     │  └─────────────┘         │           │   │
     │                          │           │   │
     └──────────────────────────┼───────────┼───┘
                                ▼           │
                      ┌──────────────────┐  │
                      │  Environment     │  │
                      │                  │  │
                      │  naisd ► helm    │  │
                      │                  │  │
                      │  Actual state    │──┘
                      │  feature @ v1.2  │ (status report)
                      └──────────────────┘
```

1. A **feature** is an OCI Helm chart with a `Feature.yaml` that declares its configurable values and target environment kinds.
2. A feature author creates an **assignment** — handing a feature version to Fasit with a set of target **labels** (e.g. `tenant=nav`, `env=prod`).
3. Fasit matches the assignment to all **environments** whose labels are a superset of the target.
4. The **reconciler** renders helm values for each matched environment and sends **deploy instructions** to the **naisd** agent running there.
5. naisd executes `helm install/upgrade` and reports status back.

When configs, environment values, or feature versions change, the reconciler detects the drift and re-deploys automatically.

## Documentation

- **[Concepts](docs/concepts.md)** — environments, features, assignments, values, reconciliation
- **[Authoring a Feature](docs/feature.md)** — Feature.yaml spec, templates, template functions

## Components

| Component | Role |
|-----------|------|
| **fasit** | HTTP/gRPC server, web UI, reconciler, deployment orchestration |
| **naisd** | Per-environment agent that executes Helm and reports status |

## Local development

```sh
mise install              # install tools
cp .env.example .env     # configure environment
mise run setup            # start postgres + pubsub emulator, seed database
mise run dev              # run fasit with auto-reload
```

See `mise run --list` for all available tasks.

## Testing

```sh
mise run test             # unit + integration tests
mise run test:unit        # unit tests only
mise run test:integration # integration tests (requires testcontainers)
```

## JSON Schema

Enable IDE autocompletion/validation for `Feature.yaml`:

```
https://storage.googleapis.com/fasit-jsonschema/feature.json
```

## License

[MIT](LICENSE)
