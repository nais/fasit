# Fasit

Multi-tenant feature management platform. Manages OCI Helm charts ("features") across environments, with deployment reconciliation and rollout orchestration via naisd agents.

## Architecture

```
cmd/
  fasit/           → Main backend (GraphQL API + UI server + workers)
  naisd/           → Agent binary (runs in tenant clusters, executes Helm)
  setup_local_env/ → Seeds local dev database
  generate_schema/ → Generates JSON schema for Feature.yaml
  tester_run/      → Integration test runner
  tester_spec/     → Generates Lua spec for integration tests

internal/
  fasit/           → App bootstrap: config, server wiring, Run()
  database/        → Postgres connection, migrations, shared repo interface
  contextloader/   → Per-request dependency injection via context.Context
  graph/           → GraphQL resolvers (gqlgen) — see internal/graph/AGENTS.md
  ui/              → Server-rendered HTML (gomponents) — see internal/ui/AGENTS.md
  deployment/      → Deployment reconciler: label-matching, deployer, status tracking — see internal/deployment/AGENTS.md
  rollout/         → Rollout reconciler: version rollouts across environments
  feature/         → Feature CRUD, template parsing, computed helm values
  environment/     → Environment CRUD, label matching
  workers/         → Background job scheduler (receiver + scheduler)
  message/         → Pub/Sub message types and publishers
  naisd/           → naisd agent logic: Helm execution, self-upgrade
  naisdstatus/     → naisd heartbeat/status tracking
  cost/            → BigQuery cost data
  audit/           → Audit log writes
  auth/            → OIDC authentication, middleware
  helm/            → Helm chart utilities
  provider/        → OCI chart provider (registry interaction)
  errs/            → Shared error types
  slack/           → Slack notifications
  server/          → HTTP server and middleware setup
  integration/     → gRPC server for naisd communication
  ioconvenience/   → IO utility functions

schema/            → GraphQL schema files (*.graphqls)
integration_tests/ → Lua-based integration tests (tester framework)
```

## Key Patterns

### Context Loader (Dependency Injection)

Domain packages expose `Register(ctx, pool)` and package-level functions that extract their querier from context. Wired in `internal/contextloader/loader.go`:

```go
// Registration (called per-request via middleware)
ctx = feature.Register(ctx, pool)
ctx = environment.Register(ctx, pool)
ctx = deployment.Register(ctx, deploymentManager)

// Usage (in resolvers, handlers — no explicit dependency passing)
feature.FeatureByNameForEnv(ctx, name, envID)
environment.GetTenant(ctx, tenantID)
```

Domain packages that use this pattern: `audit`, `cost`, `environment`, `feature`, `naisdstatus`, `deployment`.

### sqlc (Database Queries)

All SQL queries live in `queries/*.sql` files under each domain package. Generated Go code goes to `*sql/` subdirectories. Config in `sqlc.yaml` uses YAML anchors for shared settings.

| Domain | Queries dir | Generated package |
|---|---|---|
| database (shared) | `internal/database/queries` | `gensql` |
| deployment | `internal/deployment/queries` | `deploymentsql` |
| environment | `internal/environment/queries` | `environmentsql` |
| feature | `internal/feature/queries` | `featuresql` |
| rollout | `internal/rollout/queries` | `rolloutsql` |
| cost | `internal/cost/queries` | `costsql` |
| audit | `internal/audit/queries` | `auditsql` |
| naisdstatus | `internal/naisdstatus/queries` | `naisdstatussql` |

Run `mise run generate` to regenerate. All generated packages share the same migration source: `internal/database/migrations`.

### Mockery

Mocks generated via mockery v3. Config in `.mockery.yaml`. Mocks go to `{package}/mocks/` directories. Interfaces mocked: `database.Repo`, `database.Querier`, and domain-specific `Querier` interfaces.

### Code Generation

```
mise run generate        → runs all generators
  sqlc generate          → SQL → Go (sqlc.yaml)
  go run gqlgen generate → GraphQL → Go (gqlgen.yml)
  mockery                → interfaces → mocks (.mockery.yaml)
  protoc                 → protobuf → Go (schema/protobuf/)
```

Never edit files in: `gensql/`, `deploymentsql/`, `environmentsql/`, `featuresql/`, `rolloutsql/`, `costsql/`, `auditsql/`, `naisdstatussql/`, `graphgen/`, `model/donotuse/`, `mocks/`.

## Build & Run

Tool management via [mise](https://mise.jdx.dev/). Tasks in `mise/tasks/`.

```bash
mise install              # Install tools (Go 1.26, protoc, helm, etc.)
cp .env.example .env      # Configure local environment
docker-compose up -d      # Postgres + Adminer + PubSub emulator
mise run config           # Build config
mise run fasit            # Run backend
mise run naisd            # Run naisd agent
mise run test             # Run all tests (unit + integration)
mise run generate         # Run all code generation
mise run fmt              # Format code (gofumpt)
mise run check            # Lint and static analysis
```

## Testing

- Unit tests: standard `_test.go` files alongside source
- Integration tests: `testcontainers-go` for Postgres in domain package tests
- End-to-end: Lua-based tests in `integration_tests/` using [tester](https://github.com/nais/tester)
- Test helpers: `internal/database/dbtest` (DB setup), `internal/deployment/deploymenttest` (seeder)

## Database

PostgreSQL 14. Migrations in `internal/database/migrations/` (goose, embedded). Connection via pgxpool. Cloud SQL Proxy support for production.

The `database.Repo` interface composes domain-specific repo interfaces. Some domains (`deployment`, `feature`, etc.) have their own sqlc querier accessed via context loader instead.

## Deployment vs Rollout

- **Rollout**: Traditional version rollout — push a feature version across environments sequentially
- **Deployment**: Label-based deployment — deploy a feature to all environments matching target labels. Reconciler runs periodically, matching deployments to environments via `labels @> target`
- When a feature has deployments (`HasDeployments`), rollouts are effectively disabled for that feature

## Commit Convention

Semantic commit messages enforced by git hook: `feat:`, `fix:`, `refactor:`, `build:`, `test:`, `docs:`, `ci:`, `perf:`, `style:`. Scope optional: `feat(ui):`. `chore:` is NOT valid.

## Release

- **fasit**: Auto-deployed on push to main (GitHub Actions → GAR → Helm)
- **naisd**: Released by pushing a `naisd-<version>` tag
