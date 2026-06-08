# Fasit

Multi-tenant feature management platform. Manages OCI Helm charts ("features") across environments, with reconciliation and rollout orchestration via naisd agents.

## Language

**Feature**:
An OCI Helm chart with declared values (configs + computed) managed by Fasit and deployed to tenant environments.

**Computed value**:
A helm value whose output is derived from a Go template at render time, potentially referencing configs, environment values, or management values.

**Config value**:
A helm value supplied by operators. May be marked `secret`. Exists at two scopes:
- **Global config**: set once on the feature, inherited by all environments.
- **Environment config override**: set per environment, takes precedence over the global config for that environment.

**Secret taint**:
A computed value is "tainted" when its rendered output depends on at least one secret input (a secret config or a secret environment value). Tainted values are masked in the overview UI.

**Probe render**:
A second, disposable render of helm values where all secret inputs are replaced with a high-entropy sentinel. By comparing the probe output to a control render, the system identifies which computed values are tainted.

**Environment value**:
A key-value pair scoped to an environment within a tenant. May be marked `secret` at the storage layer.

**Management value**:
A key-value pair from the management environment for a tenant, available to all features via `.Management` in templates.

### Reconciliation

**Assignment**:
Desired state: a feature version bound to a set of target labels. The reconciler continuously ensures all environments whose labels are a superset of the target have this feature deployed.
_Avoid_: deployment (legacy term), release (overloaded with naisd release status)

**Deploy instruction**:
An immutable record sent to a naisd agent telling it to install or upgrade a specific feature version with specific helm values in a specific environment.

**Decision**:
The compute stage's verdict for one (assignment, environment) in a cycle: an action (deploy, skip-unchanged, skip-disabled, skip-unhealthy, fail-missing-deps, fail-missing-config, fail-render) plus a human-readable message. Recorded in `decision_log` only when it changes from the previous cycle.

**Rollout status**:
The lifecycle state of a deploy instruction: created → pending → deployed/failed. `created`/`pending` are set by the Deployer at publish; `deployed`/`failed` are reported back by naisd. Recorded in `deploy_log`; the latest entry per feature×environment is the current rollout state.
_Avoid_: deploy status, reconcile status (the legacy merged field)

**Reconciler**:
The background loop that compares desired state (assignments × environments) against actual state (latest deploy instructions) and emits new deploy instructions where the rendered helm values have changed. Runs in two stages: a pure **compute** stage that produces a decision per (assignment, environment), and a **Deployer** stage that applies them.

**Deployer**:
The reconciler's IO stage. Takes the compute stage's decisions and applies them: records them to the logs, publishes deploy instructions to naisd, and updates rollout status.
_Avoid_: dispatcher, applier (earlier names for this stage)

## Relationships

- A **Feature** has zero or more **Config values** and **Computed values**
- A **Computed value** may reference **Config values**, **Environment values**, and **Management values** in its template
- A **Computed value** acquires **Secret taint** when any referenced input is secret
- **Secret taint** is detected by comparing a **Probe render** against a control render
- An **Assignment** targets a **Feature** version at a set of **Environment labels**; the most specific label match wins per environment
- The **Reconciler** produces a **Deploy instruction** for each (assignment, environment) pair whose rendered helm values have changed
- The compute stage emits one **Decision** per (assignment, environment); the **Deployer** applies it
- The displayed status of a feature×environment is two complementary signals: its **Rollout status** (from `deploy_log`) and its latest **Decision** (from `decision_log`)

## Example dialogue

> **Dev:** "If a computed value uses `{{ .Env.token | b64enc }}`, is it tainted?"
> **Domain expert:** "Yes — if `token` is a secret environment value, the probe render substitutes a sentinel, so the b64enc output differs between control and probe. The diff flags `out` as tainted."

> **Dev:** "What if the template just checks `{{ if .Env.token }}static{{ end }}`?"
> **Domain expert:** "Not tainted — both renders produce `static` because the sentinel is truthy too. The output is identical."

## Flagged ambiguities

- "secret" was used to mean both "a config/env value declared as secret" and "a computed value whose output depends on a secret input." Resolved: the input property is **secret** (on config/env values); the derived property on computed values is **secret taint**.
