# Bulk-fetch reconciler with parallel render

The deployment reconciler was rewritten from a sequential per-environment, per-deployment loop into a four-phase pipeline: bulk fetch → build lookup maps → parallel render → bulk write + publish. The new implementation lives in `internal/reconciler` with its own sqlc queries that read across all domain tables directly.

## Considered options

**Keep per-environment sequential loop, just cache within a cycle.** Simplest change — add maps for env values, configs, health. Still O(environments × deployments) DB queries for shouldDeploy checks and deploy instruction creation. Rejected because it doesn't address the fundamental query-per-deployment scaling problem (1167+ extra queries per cycle observed in production).

**Parallelize environments but keep per-deployment queries.** Bounded worker pool processes environments concurrently, each still querying per deployment. Improves wall-clock time but increases DB connection contention on a pool of 5. Rejected because it trades latency for throughput pressure on the DB.

**Bulk fetch all data, render in pure goroutines, bulk write results.** All reads happen upfront (~8 queries total regardless of deployment count). Render goroutines are pure — no DB connection needed. All writes happen after rendering via unnest-based bulk inserts. Chosen because it decouples compute from IO, makes the DB load constant, and allows unlimited render parallelism.

## Key decisions

- **`internal/reconciler` owns all its queries.** Reads from deployments, feature_data, configurations_*, environment_values, deploy_instructions, health_statuses, disabled_features directly. Does not import domain package query functions. May import `internal/feature` for pure render functions only.
- **Deployment matching in Go.** All deployments (latest per feature+target) are fetched once. Label containment (`labels ⊇ target`) and most-specific selection happen in Go per environment.
- **Pre-generated UUIDs.** Deploy instruction IDs are generated in Go before bulk insert, so they're available for pub/sub messages without RETURNING.
- **Unnest-based bulk writes** for both deploy_instructions and deployment_statuses (upsert). Single query per table per reconcile cycle.
