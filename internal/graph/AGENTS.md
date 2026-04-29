# GraphQL Layer

GraphQL API powered by [gqlgen](https://gqlgen.com/). Schema-first: write `.graphqls` files in `schema/`, generate Go boilerplate, implement resolvers by hand.

## Directory Structure

```
schema/                → GraphQL schema files (*.graphqls)
internal/graph/
  graphgen/            → GENERATED: execution runtime (DO NOT EDIT)
  model/               → Hand-written model types used by resolvers
    donotuse/          → GENERATED: model stubs (DO NOT EDIT — use model/ or autobind instead)
  scalars/             → Custom scalar implementations (UUID, RawMessage, EnvironmentLabels)
  resolver.go          → Resolver struct: dependency injection root (hand-written)
  *.resolvers.go       → Resolver implementations, one per schema domain (hand-written)
  log_notifier.go      → Postgres LISTEN/NOTIFY for log streaming
  updates_notifier.go  → Postgres LISTEN/NOTIFY for deploy instruction updates
```

## Key Facts

- **Config**: `gqlgen.yml`. Resolver layout: `follow-schema` (one file per schema file). Autobind: `deployment`, `environment`, `graph/model`.
- **Custom scalars**: `ID` → `scalars.UUID`, `RawMessage` → `scalars.RawMessage`, `EnvironmentLabels` → `scalars.EnvironmentLabels`.
- **Data access**: Resolvers use context-loaded packages (`feature.X(ctx)`, `deployment.X(ctx)`) or `r.Repo` for operations not yet migrated.
- **Subscriptions**: Real-time via Postgres LISTEN/NOTIFY (`log_notifier.go`, `updates_notifier.go`).
- **Adding a domain**: Create `schema/X.graphqls` → `mise run generate` → implement stubs in `internal/graph/X.resolvers.go`. New Go types go in `model/` (not `model/donotuse/`).
