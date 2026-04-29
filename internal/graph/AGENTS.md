# GraphQL Layer

GraphQL API powered by [gqlgen](https://gqlgen.com/). Schema-first: write `.graphqls` files, generate Go boilerplate, implement resolvers by hand.

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
  *_test.go            → Resolver tests
  log_notifier.go      → Postgres LISTEN/NOTIFY for log streaming
  updates_notifier.go  → Postgres LISTEN/NOTIFY for deploy instruction updates
  metrics.go           → GraphQL operation metrics
  playground_helpers.go → Playground feature helpers
```

## Generated vs Hand-Written

| Path | Status | Notes |
|---|---|---|
| `graphgen/graphgen.go` | GENERATED | gqlgen execution runtime. Never edit. |
| `model/donotuse/models_gen.go` | GENERATED | Type stubs. Import from `model/` instead. |
| `resolver.go` | Hand-written | Resolver struct with injected dependencies |
| `*.resolvers.go` | Hand-written | Resolver method implementations |
| `scalars/` | Hand-written | Custom scalar marshaling |

Regenerate with `mise run generate`.

## Configuration (gqlgen.yml)

```yaml
schema: schema/*.graphqls
exec:
  filename: internal/graph/graphgen/graphgen.go
resolver:
  layout: follow-schema        # One resolver file per schema file
  dir: internal/graph
  omit_template_comment: true
autobind:                       # Auto-match Go types to GraphQL types
  - github.com/nais/fasit/internal/deployment
  - github.com/nais/fasit/internal/environment
  - github.com/nais/fasit/internal/graph/model
```

**Autobind**: gqlgen automatically maps GraphQL types to Go types in the listed packages. If a Go struct matches a GraphQL type name, no code generation is needed for that type. Add new types to `internal/graph/model/` or the relevant domain package and they'll be picked up.

## Resolver Pattern

Resolvers follow a consistent pattern:

```go
// In deployment.resolvers.go (follows schema/deployments.graphqls)
func (r *queryResolver) Deployments(ctx context.Context) ([]*model.Deployment, error) {
    // Use context-loaded packages for data access
    return deployment.ListDeployments(ctx)
}

func (r *mutationResolver) CreateDeployment(ctx context.Context, input model.CreateDeploymentInput) (*model.Deployment, error) {
    // Mutations often use r.Repo (shared repo) for cross-domain operations
    return r.Repo.CreateDeployment(ctx, ...)
}
```

Data access in resolvers uses:
1. **Context-loaded packages** (preferred): `feature.X(ctx, ...)`, `deployment.X(ctx, ...)`
2. **Shared repo** (`r.Repo`): For operations not yet migrated to context-loaded packages

## Adding a New Schema Domain

1. Create `schema/newdomain.graphqls` with types, queries, mutations
2. Run `mise run generate`
3. gqlgen creates `internal/graph/newdomain.resolvers.go` with stub methods
4. Implement the resolver methods
5. If new Go types are needed, add to `internal/graph/model/` (not `model/donotuse/`)

## Custom Scalars

Defined in `scalars/` package, mapped in `gqlgen.yml`:
- `ID` → `scalars.UUID` (wraps `google/uuid`)
- `RawMessage` → `scalars.RawMessage` (raw JSON)
- `EnvironmentLabels` → `scalars.EnvironmentLabels` (label map)

## Subscriptions

Real-time updates via Postgres LISTEN/NOTIFY:
- `log_notifier.go` — streams naisd log entries
- `updates_notifier.go` — streams deploy instruction changes

Both use `internal/database/notifier.Notifier` for Postgres channel management.
