# Fasit

Multi-tenant feature management platform. Manages OCI Helm charts ("features") across environments, with deployment reconciliation and orchestration via naisd agents.

## Architecture

```
cmd/
  fasit/           → Main backend (REST API + UI server + workers)
  naisd/           → Agent binary (runs in tenant clusters, executes Helm)
  setup_local_env/ → Seeds local dev database
  generate_schema/ → Generates JSON schema for Feature.yaml

internal/
  fasit/           → App bootstrap: config, server wiring, Run()
  database/        → Postgres connection, migrations, shared repo interface
  contextloader/   → Per-request dependency injection via context.Context
  ui/              → Server-rendered HTML (gomponents) — see internal/ui/AGENTS.md
  deployment/      → Deployment reconciler — see internal/deployment/AGENTS.md
  feature/         → Feature CRUD, template parsing, computed helm values
  environment/     → Environment CRUD, label matching
  workers/         → Background job scheduler (receiver + scheduler)
  message/         → Pub/Sub message types and publishers
  naisd/           → naisd agent logic: Helm execution, self-upgrade
  naisdstatus/     → naisd heartbeat/status tracking
  provider/        → OCI chart provider (registry interaction)
  integration/     → gRPC server for naisd communication
  ioconvenience/

schema/            → JSON schema for Feature.yaml validation / autocomplete, and protobuf definitions for gRPC
```

## Key Patterns

### Context Loader (Dependency Injection)

Domain packages expose `Register(ctx, pool)` and package-level functions that extract their querier from context. Wired in `internal/contextloader/loader.go`. Packages using this: `audit`, `environment`, `feature`, `naisdstatus`, `deployment`.

### sqlc

SQL queries in `queries/*.sql` per domain package. Generated Go in `*sql/` subdirectories. Config in `sqlc.yaml` (YAML anchors for shared settings). All share migration source: `internal/database/migrations`.

New queries go in the domain package that owns the data (`internal/deployment/queries/`, `internal/feature/queries/`, `internal/environment/queries/`, etc). `internal/database/queries/` is a legacy shared location — avoid adding new queries there; move them to the owning domain package when practical.

### Code Generation

`mise run generate` runs: sqlc, protoc.

Never edit files in: `gensql/`, `deploymentsql/`, `environmentsql/`, `featuresql/`, `auditsql/`, `naisdstatussql/`.

## Build & Run

Tool management via mise. Tasks in `mise/tasks/`. Key commands: `mise run fasit`, `mise run test`, `mise run generate`, `mise run check`.

## Testing

- Unit tests: `_test.go` alongside source
- Integration: `testcontainers-go` for Postgres
- End-to-end: Lua-based in `integration_tests/` using [tester](https://github.com/nais/tester)

Never modify or delete existing tests — especially integration tests in `integration_tests/` and any code behind the `integration_test` build tag — without first consulting the user. Tests encode intended behavior; if a change appears to require touching a test, ask before doing so. This also applies to seemingly "unused" test helpers: code reachable only from build-tagged tests can look dead to static analysis but is not.

## Database

PostgreSQL 14. Migrations in `internal/database/migrations/` (goose, embedded). `database.Repo` composes domain-specific repo interfaces; some domains have their own sqlc querier via context loader instead.

## Code Style

### Comments

Prefer self-documenting code: clear names, small functions, obvious control flow. Only add a comment when absolutely necessary — i.e. when the code cannot reasonably explain itself, such as a non-obvious side effect, a subtle correctness constraint, or a required workaround. Do not restate what the code does. Godoc on exported declarations is fine when it adds context beyond the name and signature; otherwise omit it.

### Error Handling

Never ignore errors with `_ =` unless absolutely necessary (e.g. best-effort cleanup in a defer). Always propagate or handle errors explicitly.

## Commit Convention

Semantic messages enforced by hook: `feat:`, `fix:`, `refactor:`, `build:`, `test:`, `docs:`, `ci:`, `perf:`, `style:`. Scope optional: `feat(ui):`. `chore:` is NOT valid.

Before committing, always run `mise run generate`, `mise run fmt`, and `mise run check`.

## Release

- **fasit**: Auto-deployed on push to main
- **naisd**: Push `naisd-<version>` tag
