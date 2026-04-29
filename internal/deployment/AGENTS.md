# Deployment Package

Label-based deployment orchestrator. Deploys features to environments matching target labels, with periodic reconciliation and naisd agent communication.

## Directory Structure

```
deployment/
  manager.go       → Manager: wires reconciler + deployer, exposes Run/Reconcile/Receive
  reconciler.go    → Periodic reconciliation loop + manual trigger via channel
  deployer.go      → Creates deployments, publishes deploy instructions to naisd
  handler.go       → HTTP API for GitHub Actions (CreateDeployment, GetDeployment, OIDC auth)
  model.go         → Domain types (Deployment, DeploymentStatus) + SQL→domain conversion
  queries.go       → Context loader (Register/GetManager) + public package API
  deploymentsql/   → GENERATED: sqlc querier (DO NOT EDIT)
  deploymenttest/  → Test helpers: Seeder for integration tests
  queries/         → SQL query source files for sqlc
```

## Runtime Flow

```
GitHub Actions → handler.go (OIDC) → CreateDeployment → deployer
                                                            ↓
reconciler (periodic/triggered) → deployer.deployToEnvironment
                                      ↓
                              publish DeployInstruction → naisd
                                                            ↓
                              Manager.Receive ← naisd status message
                                      ↓
                              SetDeploymentStatus (DB)
```

## Key Components

### Manager

Constructor: `NewManager(pool, publisherFactory, meter, log)`. Wires deployer and reconciler internally.

- `Run(ctx, interval)` — starts the reconciler loop
- `Reconcile(ctx)` — synchronous reconciliation
- `Receive(ctx, *message.Helm)` — inbound naisd status handler; maps DIID → deployment status in DB

### Reconciler

Runs periodically or on manual trigger. For each tenant environment:
1. `ListDeploymentsToReconcile` (SQL: `labels @> target AND enabled = TRUE`)
2. `filterDeployments` — when multiple deployments target same feature, pick most specific target (longest label map), then latest
3. Call `deployer.deployToEnvironment` for each

Concurrency: `sync.Mutex.TryLock` prevents overlapping reconcile runs. Trigger channel is buffered(1) for dedup.

### Deployer

- `deployToEnvironment` — register env feature, health-check naisd, compute helm values, generate hash (sha256 of values+version+chart), skip if unchanged, create deploy instruction in DB, publish message
- `deployToCI` — find CI environments for target labels, deploy to each, wait for statuses
- Dependency check: queries deploy_instructions history to verify all feature dependencies are deployed

### Handler (HTTP API)

`NewHttpHandler` exposes REST endpoints for GitHub Actions:
- `CreateDeployment` — validates GitHub Actions OIDC token (issuer: `token.actions.githubusercontent.com`, owner: `nais`), creates deployment
- `GetDeployment` — returns deployment by ID

Uses `programContext` to prevent cancellation on client disconnect (deployments are long-running).

## Public API

Package-level functions accessed via context loader:

```go
Register(ctx, *Manager) context.Context
CreateDeployment(ctx, Request) (uuid.UUID, error)
GetDeployment(ctx, uuid.UUID) (*Deployment, error)
ListDeployments(ctx) ([]*Deployment, error)
ListDeploymentsByFeature(ctx, featureName) ([]*Deployment, error)
ListDeploymentStatuses(ctx, deploymentID) ([]*DeploymentStatus, error)
DeleteDeployment(ctx, deploymentID) error
DeleteDeploymentsByFeatureAndTarget(ctx, featureName, target, ci) error
TriggerReconcile(ctx, event) chan TriggerResult
```

## Implementation Details

- **Hash-based dedup**: `generateHash` serializes helm values + version + chart → sha256. Deploy instruction skipped if hash matches existing in-progress instruction.
- **Publisher factory**: `NewPublisher` func creates a Publisher per environment topic (`naisd-{tenant}-{env}`). Publisher.Stop() called via defer.
- **Trigger pattern**: `reconciler.trigger()` returns `chan TriggerResult` — callers can wait synchronously for reconcile outcome.
- **Dependency semantics**: A dependency is "satisfied" if it has any successful deploy instruction in history (not a state machine — history-based check).
