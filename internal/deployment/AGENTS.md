# Deployment Package

Label-based deployment orchestrator. Deploys features to environments matching target labels, with periodic reconciliation and naisd agent communication.

## Structure

```
manager.go       → Wires reconciler + deployer, exposes Run/Reconcile/Receive
reconciler.go    → Periodic reconciliation loop + manual trigger via channel
deployer.go      → Creates deployments, publishes deploy instructions to naisd
handler.go       → HTTP API for GitHub Actions (OIDC auth, CreateDeployment)
model.go         → Domain types + SQL→domain conversion helpers
queries.go       → Context loader (Register/GetManager) + public package API
deploymentsql/   → GENERATED (DO NOT EDIT)
deploymenttest/
queries/
```

## Flow

```
GitHub Actions → handler.go (OIDC) → CreateDeployment → deployer
                                                            ↓
reconciler (periodic/triggered) → deployer.deployToEnvironment
                                      ↓
                              publish DeployInstruction → naisd
                                                            ↓
                              Manager.Receive ← naisd status → SetDeploymentStatus
```

## Key Details

- **Manager** wires reconciler + deployer. `Run(ctx, interval)` starts reconcile loop. `Receive(ctx, msg)` handles inbound naisd statuses.
- **Reconciler**: For each env, `ListDeploymentsToReconcile` (SQL: `labels @> target AND enabled = TRUE`), `filterDeployments` (most specific target wins, then latest), deploy each. `TryLock` prevents concurrent runs. Trigger channel buffered(1) for dedup.
- **Deployer**: Computes helm values, generates sha256 hash (values+version+chart), skips if unchanged, creates deploy instruction, publishes to naisd. Dependency check is history-based (any past successful deploy satisfies).
- **Handler**: Validates GitHub Actions OIDC tokens (issuer: `token.actions.githubusercontent.com`, owner: `nais`). Uses `programContext` to survive client disconnect.
- **Publisher factory**: `NewPublisher` func creates a Publisher per environment topic. Lifecycle: `defer publisher.Stop()`.
