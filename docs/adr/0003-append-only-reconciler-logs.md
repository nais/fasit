# Append-only logs replace mutable reconciler status tables

The reconciler's per-feature×environment state was tracked in two mutable tables — `feature_reconcile_statuses` (a merged "reconcile status" updated in place by both the reconciler and the naisd callback) and `deploy_instructions` (one row per deploy, `status` UPDATEd as naisd reports back). We are replacing both with two append-only logs plus derived `DISTINCT ON` views:

- **`decision_log`** / `decision_status` — the compute stage's decision per (assignment, environment): an `action` (deploy / skip-* / fail-*) plus `message`. A row is inserted only when `(feature_assignment_id, feature_version, action, message)` differs from the latest row for `(environment_id, feature_name)`; change detection is done in Go, not SQL, for speed.
- **`deploy_log`** / `deploy_status` — the deploy lifecycle. The Deployer appends a row at publish (`sent`) carrying the DIID, hash, and rendered values; the naisd Receiver appends new rows with the same DIID for `installing` and the terminal `deployed`/`failed`. Latest row per `(environment_id, feature_name)` is the current rollout state.

The displayed status of a feature×environment becomes **two complementary signals** rather than one merged field: rollout state from `deploy_status` and the latest reconciler decision from `decision_status`.

## Why

- Mutable status tables lose history and conflate two distinct facts (what the reconciler decided vs. what naisd reported). Append-only logs keep the full timeline and separate the two concerns.
- Append-only writes suit the bulk-fetch/parallel-render reconciler (ADR 0002): no in-place upserts to serialize, and the naisd round-trip becomes an append correlated by DIID.
- `decision_log` records only *changes*, so the log stays small despite running every cycle.

## Considered options

- **Keep the mutable tables, just add the logs for evaluation.** Where we started. Rejected as the end state: two sources of truth, and the logs add cost without removing anything.
- **One unified `reconcile_log` with a `kind` discriminator.** Rejected — the two logs have genuinely different shapes (decision: `action`+`message`; deploy: `status`+`hash`+`values`) and different write cardinalities; nullable phase-specific columns muddy the model.
- **Backfill history on cutover.** Rejected for now — mapping mutable status rows into append-only lifecycle rows is fiddly; current status repopulates within one reconcile cycle. History continuity can be a later follow-up.

## Transition

Hard cutover, no dual-write. The new write path (Deployer + Receiver append to the logs), the new read path (UI/API read the `*_status` views), and the removal of `deploy_instructions` + `feature_reconcile_statuses` all land together. No historical backfill: current status repopulates within one reconcile cycle. `logs.deploy_instruction` loses its FK and becomes a plain DIID correlation column, since DIID now repeats across transition rows.

The `message.DeployInstruction` wire type and the naisd agent code that consumes it are unchanged — we remove the *persistence* of deploy instructions, not the act of sending them. `release_statuses` (naisd's full helm-release inventory per environment) is also unaffected; it is not tied to a deploy instruction and is not part of this change.

## Affected behaviours

- **Manual redeploy removed.** The old "redeploy" button blanked `deploy_instructions.hash` to force a redeploy. There is no row to mutate in an append-only log, so the manual redeploy/invalidate affordance (`InvalidateLatestDeploy`) is dropped; redeploys are driven by config/version changes. `featureassignment`'s now-dead `CreateDeployInstruction`/`UpdateDeployInstructionStatus` queries are removed too.
- **Deploy history reads `deploy_log`.** The per-deploy history on the assignment-detail and environment-feature pages (previously `deploy_instructions` via the `feature`/`uidata` queries) is rebacked by `deploy_log`: the publish rows carry version/hash/values and the appended transition rows give the full sent→installing→deployed/failed timeline. `logs` keeps its DIID column for correlation.
