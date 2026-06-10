# fasitd

`fasitd` is an experimental replacement for the `naisd` deploy transport. Instead
of receiving deploy instructions over Pub/Sub, a `fasitd` agent opens a
**long-lived gRPC session** to Fasit and receives commands on that stream,
reporting status, logs, and Helm release inventory back over the same stream.

It currently runs in **dry-run**: it never executes Helm. It exists to validate
the replacement transport and agent protocol alongside the canonical `naisd`
rollout, without mixing experimental results into the real rollout state. See
[ADR 0004](adr/0004-fasitd-dry-run-grpc-lane.md) for the decision record.

## Terminology

- **fasitd session** — the long-lived bidirectional gRPC stream from one `fasitd`
  agent to Fasit, keyed by tenant + environment. Distinct from a Pub/Sub
  *subscription*.
- **fasitd command** — a deploy/uninstall instruction Fasit sends over a session.
  Carries the same logical payload as a naisd `DeployInstruction` (feature name,
  version, chart, config hash, timeout, values, uninstall flag).

## How it relates to naisd

During dry-run both lanes run from the **same reconcile decisions**:

```
                       ┌──────────────────────────────┐
 reconciler decision → │        MultiDeployer         │
                       │  primary: naisd (Pub/Sub) ────┼──► canonical rollout (deploy_log)
                       │  secondary: fasitd (gRPC) ────┼──► shadow lane (fasitd_* tables)
                       └──────────────────────────────┘
```

- The **naisd Pub/Sub lane stays canonical**: its failure fails the reconcile cycle.
- The **fasitd lane is best-effort**: errors are logged, never propagated, and
  never block the real rollout.
- `"deployed"` in the fasitd lane means **the dry-run succeeded**, not that Helm
  installed a release.

## Lifecycle of a fasitd command

1. The reconciler emits deploy decisions. `MultiDeployer` runs naisd first, then
   the fasitd `Deployer`.
2. For each `ActionDeploy` decision, the fasitd `Deployer` writes an immutable
   `fasitd_commands` row (keyed by a generated `diid`).
3. Delivery:
   - **Active session** → the command is pushed onto the session stream and
     recorded as `sent`.
   - **No session / closed / send timeout** → recorded as `undeliverable`. Fasit
     does **not** queue or replay it; only new commands are sent after reconnect.
4. The agent acknowledges (`Ack`) → Fasit records `installing`.
5. The agent reports a terminal `Status` → Fasit records `deployed` or `failed`.
6. The agent streams `Logs` (per `diid`) and periodic `Releases` inventory.

Commands are handled **serially per environment** (one agent = one environment =
one in-flight command), matching naisd's synchronous receive model.

## Session rules

- One active session per tenant + environment. A second connection for the same
  key is **rejected** (`AlreadyExists`). More sophisticated handover is future work.
- A session registers with: tenant, environment, fasitd version, protocol version.
- Session liveness **is** the health signal — there is no separate health message
  or table during dry-run.

## Auth

The fasitd gRPC server is a **dedicated, IAP-protected listener** on its own port
(`FASITD_GRPC_BIND_ADDRESS`, default `:4445`), separate from the existing provider
gRPC server. A stream interceptor validates the Google IAP JWT assertion
(`x-goog-iap-jwt-assertion`: issuer, audience, issued-at). When
`INSECURE_SKIP_PROXY` is set (local dev), validation is bypassed.

Agents authenticate through IAP using an OIDC ID token (audience = IAP OAuth
client ID) as per-RPC credentials; IAP then injects the assertion header the
server validates.

## Code map

### Protocol
- `schema/protobuf/fasitd.proto` — the `Fasitd.Connect` bidirectional stream.
  `AgentMessage` (Register/Ack/Status/Logs/Releases) flows up; `ServerMessage`
  (Command) flows down.
- `internal/fasitd/protogen/` — generated Go (do not edit; `mise run generate`).

### Persistence (`fasitd_*`, isolated from canonical `deploy_log`)
- `internal/database/migrations/0095_fasitd.sql`:
  - `fasitd_commands` — immutable command record, keyed by `diid`
  - `fasitd_command_statuses` (+ `fasitd_command_status` view) — append-only
    lifecycle: `sent → installing → deployed/failed`, plus `undeliverable`
  - `fasitd_helm_logs` — agent log lines per `diid`
  - `fasitd_release_statuses` — latest Helm inventory per environment × feature
- `internal/fasitd/queries/*.sql` → generated `internal/fasitd/fasitd_sql/` (do not edit).

### Server (inside the Fasit process)
- `internal/fasitd/registry.go` — in-memory session registry; rejects a second
  session per env; `remove` only evicts the matching session object.
- `internal/fasitd/server.go` — implements `Connect`: handshake, env resolution,
  send goroutine + receive loop, ingest of agent messages into `fasitd_*`.
- `internal/fasitd/auth.go` — `NewGrpcServer` + the IAP stream interceptor.
- `internal/fasitd/deployer.go` — the shadow `Deployer` (records command,
  delivers or marks `undeliverable`, best-effort).
- `internal/reconciler/multideployer.go` — `MultiDeployer`: canonical primary +
  best-effort secondaries.
- Wired in `internal/fasit/run.go` / `internal/fasit/config.go`.

### Agent
- `internal/fasitd/agent.go` — client: register, serial command handling
  (ack → log → `deployed`, no Helm), optional periodic release inventory.
- `cmd/fasitd/` — the `fasitd` binary (dial, reconnect loop, `/metrics`).

### Packaging / ops
- `Dockerfile_fasitd` — builds the `fasitd` image (includes Helm for read-only
  release listing). Built/pushed by CI like `Dockerfile_naisd`.
- `charts/fasitd/` — Helm chart (Deployment, ServiceAccount, ClusterRoleBinding,
  NetworkPolicy, `Feature.yaml`, `values.yaml`).
- `mise/tasks/build/fasitd.sh` — builds `./bin/fasitd`.
- `mise/tasks/fasitd-agent.sh` — runs a local agent: `mise run fasitd-agent <tenant> <env>`.

## Metrics

- `fasitd_commands_total{outcome,tenant,environment,feature}` — dispatch outcome
  (`sent` / `undeliverable`). Use for **delivery parity**: did each real deploy
  decision also produce a deliverable fasitd command?
- `fasitd_reports_received_total{type}` — reports ingested from agents
  (`ack` / `status` / `logs` / `releases`).

## Inspecting results

No UI exists for the dry-run. Inspect via metrics or SQL, e.g.:

```sql
-- Latest state of every fasitd command
SELECT * FROM fasitd_command_status ORDER BY created_at DESC;

-- Commands that never reached a session
SELECT * FROM fasitd_command_status WHERE status = 'undeliverable';
```

## Known limitations / follow-ups

- **Dry-run only**: the agent never executes Helm; it will be replaced by the
  real execution path later.
- **Self-upgrade** is not implemented (naisd has it); future work.
- **Uninstall** is wired through the protocol and persistence, but Fasit does not
  currently emit uninstall decisions, so commands always dispatch with
  `uninstall = false`.
- **RBAC**: `charts/fasitd` currently grants `cluster-admin` (copied from naisd's
  form). Dry-run fasitd only needs read access for `helm list`; tighten before any
  real deploy.
- **Cutover**: `fasitd_*` is shaped for dry-run now; the canonical migration
  strategy is a separate, later design step.
