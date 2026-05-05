# Fasit

Multi-tenant feature management platform. Manages OCI Helm charts ("features") across environments, with deployment reconciliation and rollout orchestration via naisd agents.

## Architecture

```
cmd/
  fasit/           → Main backend (GraphQL API + UI server + workers)
  naisd/           → Agent binary (runs in tenant clusters, executes Helm)
  setup_local_env/ → Seeds local dev database
  generate_schema/ → Generates JSON schema for Feature.yaml
  tester_run/
  tester_spec/     → Generates Lua spec for integration tests

internal/
  fasit/           → App bootstrap: config, server wiring, Run()
  database/        → Postgres connection, migrations, shared repo interface
  contextloader/   → Per-request dependency injection via context.Context
  graph/           → GraphQL resolvers (gqlgen) — see internal/graph/AGENTS.md
  ui/              → Server-rendered HTML (gomponents) — see internal/ui/AGENTS.md
  deployment/      → Deployment reconciler — see internal/deployment/AGENTS.md
  rollout/         → Rollout reconciler: version rollouts across environments
  feature/         → Feature CRUD, template parsing, computed helm values
  environment/     → Environment CRUD, label matching
  workers/         → Background job scheduler (receiver + scheduler)
  message/         → Pub/Sub message types and publishers
  naisd/           → naisd agent logic: Helm execution, self-upgrade
  naisdstatus/     → naisd heartbeat/status tracking
  cost/            → BigQuery cost data
  provider/        → OCI chart provider (registry interaction)
  integration/     → gRPC server for naisd communication
  ioconvenience/

schema/            → GraphQL schema files (*.graphqls)
integration_tests/ → Lua-based integration tests (tester framework)
```

## Key Patterns

### Context Loader (Dependency Injection)

Domain packages expose `Register(ctx, pool)` and package-level functions that extract their querier from context. Wired in `internal/contextloader/loader.go`. Packages using this: `audit`, `cost`, `environment`, `feature`, `naisdstatus`, `deployment`.

### sqlc

SQL queries in `queries/*.sql` per domain package. Generated Go in `*sql/` subdirectories. Config in `sqlc.yaml` (YAML anchors for shared settings). All share migration source: `internal/database/migrations`.

### Code Generation

`mise run generate` runs: sqlc, gqlgen, mockery (v3, config in `.mockery.yaml`), protoc.

Never edit files in: `gensql/`, `deploymentsql/`, `environmentsql/`, `featuresql/`, `rolloutsql/`, `costsql/`, `auditsql/`, `naisdstatussql/`, `graphgen/`, `model/donotuse/`, `mocks/`.

## Deployment vs Rollout

- **Rollout**: Push a feature version across environments sequentially (ordered progression)
- **Deployment**: Label-based — deploy to all environments matching target labels (`labels @> target`), reconciled periodically
- When a feature has deployments, rollouts are effectively disabled for that feature
- Both produce `deploy_instructions` for naisd and share `model.RolloutStatus` enum / `model.RolloutLog` type at the domain layer — this is intentional (naisd doesn't distinguish)
- UI, API, and domain packages keep rollouts and deployments fully separate — do NOT mix them

## Build & Run

Tool management via mise. Tasks in `mise/tasks/`. Key commands: `mise run fasit`, `mise run test`, `mise run generate`, `mise run check`.

## Testing

- Unit tests: `_test.go` alongside source
- Integration: `testcontainers-go` for Postgres
- End-to-end: Lua-based in `integration_tests/` using [tester](https://github.com/nais/tester)

## Database

PostgreSQL 14. Migrations in `internal/database/migrations/` (goose, embedded). `database.Repo` composes domain-specific repo interfaces; some domains have their own sqlc querier via context loader instead.

## Code Style

### Comments

Keep code comments to an absolute minimum. Only add a comment when the code cannot reasonably explain itself — e.g. a non-obvious side effect, a subtle correctness constraint, or a required workaround. Do not restate what the code does. Godoc comments on exported functions are fine when they add context beyond the name and signature.

## Commit Convention

Semantic messages enforced by hook: `feat:`, `fix:`, `refactor:`, `build:`, `test:`, `docs:`, `ci:`, `perf:`, `style:`. Scope optional: `feat(ui):`. `chore:` is NOT valid.

## Release

- **fasit**: Auto-deployed on push to main
- **naisd**: Push `naisd-<version>` tag
