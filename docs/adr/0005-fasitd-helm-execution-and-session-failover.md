# Fasitd executes Helm itself and keeps an ordered list of sessions per environment

ADR 0004 introduced `fasitd` as a dry-run lane. We are now letting `fasitd` run real Helm operations, still behind a `--dry-run` switch (default on, settable per environment through the chart's `dryRun` value), and letting several `fasitd` replicas connect for the same environment.

- **Ordered sessions, first is primary.** Fasit keeps the connected **Fasitd sessions** for an environment as an ordered list in connection order. Index 0 is the primary; commands are offered to sessions in order and go to the first that accepts. Only the primary's release inventory is persisted.
- **Draining.** A session that will not take new commands — on shutdown, or before upgrading itself — sends `Draining`. Fasit removes it from routing and marks any commands still queued for it as `undeliverable`, but keeps the stream open so an in-flight command can still report logs and a terminal status. The agent keeps reading briefly after draining and rejects anything still on the wire as `undeliverable`.
- **Self-upgrade in place.** A command for the `fasitd` release itself runs `helm upgrade --atomic --wait` from inside the running pod. The new pod only becomes ready (`/readyz`) once it has registered with Fasit; if it never does, Helm rolls back. A release left pending by an interrupted operation is recovered (rolled back, or removed if never installed) once it is older than the command timeout.

## Why

- A separate upgrade Job (naisd's approach) decouples the upgrade from the pod but needs its own lifecycle, cleanup and status plumbing back to Fasit. Running Helm in-process reuses the session for logs and status, and `--atomic` plus a registration-gated readiness probe gives the same safety.
- Several replicas remove the gap where an environment has no session during a rolling update or pod loss.
- "First in the list is primary" needs no extra state: failover is just trying the next entry, and connection order keeps a stable pod primary instead of a flapping one.

## Considered options

- **Reject a second session per environment** (the ADR 0004 behaviour). Rejected: makes any rolling update of `fasitd` drop commands.
- **Newest session wins.** Rejected: a restarting pod would keep stealing the primary role.
- **Explicit active/standby flags.** Rejected as unnecessary state; list order already encodes it.
- **Round-robin across sessions.** Rejected: concurrent Helm operations on the same release fail with "another operation is in progress".
