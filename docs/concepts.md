# Concepts

This document explains the core concepts in Fasit and how they relate to each other.

## Environments

An **environment** represents a target where features can be deployed — a Kubernetes cluster belonging to a tenant. Each environment has:

- A set of **labels**: key-value pairs describing the environment (e.g. `tenant=nav`, `env=prod`, `region=europe-north1`)

Labels are the mechanism for targeting. They let you express "deploy this to all production environments" or "deploy this only to tenant X" without enumerating individual environments.

Each environment also holds **environment values** — key-value pairs available to feature templates at render time (e.g. cluster name, domain, project ID). These can be marked as **secret**.

## Features

A **feature** is an OCI Helm chart with a `Feature.yaml` file alongside the `Chart.yaml`. The `Feature.yaml` declares which **values** Fasit should manage (as opposed to static values baked into the chart)

See [Authoring a Feature](feature.md) for the full spec.

### Values

Fasit manages helm values on behalf of features. Each managed value is one of:

**Config values** — operator-supplied inputs. Set globally on the feature or overridden per environment. Can be marked `secret`.

**Computed values** — derived at render time from a Go template. Templates can reference:
- `.Env.*` — environment values
- `.Config.*` — config values
- `.Envs` — all environments (for cross-environment lookups)

**Mixed values** — both config and computed are defined. The computed template is used unless an explicit config value is set.

## Assignments

An **assignment** is how a user brings a feature into Fasit's control. By creating an assignment, you declare part of the desired state: "this feature version should run in all environments matching these labels."

The full desired state for a given (feature, environment) pair is the assignment plus its **configuration** — the resolved set of helm values drawn from:

- **Environment values** — key-value pairs set on the environment (e.g. cluster name, domain)
- **Global config** — set once on the feature, inherited by all environments
- **Environment config overrides** — per-environment overrides of global config
- **Computed values** — derived at render time from templates referencing the above

The reconciler continuously ensures the actual state in each matched environment converges with this desired state.

### Label matching

An environment matches an assignment when the environment's labels are a **superset** of the assignment's target labels:

```
Assignment target: {tenant: nav, env: prod}
Environment labels: {tenant: nav, env: prod, region: europe-north1}  ✓ matches
Environment labels: {tenant: nav, env: dev}                          ✗ no match
```

### Specificity

When multiple assignments of the same feature match an environment, the **most specific** one wins — the one with the most target labels. This lets you set a general assignment and override it for specific environments:

```
Assignment A target: {tenant: nav}              → version 1.0 (general)
Assignment B target: {tenant: nav, env: prod}   → version 1.1 (prod override)
```

An environment with `{tenant: nav, env: prod, ...}` gets version 1.1.

## Reconciliation

The **reconciler** is the background loop that turns desired state into actual state. It runs periodically and:

1. For each environment, finds all matching assignments (label superset check)
2. Resolves specificity conflicts (most labels wins)
3. Renders helm values for each (feature version, environment) pair
4. Compares the rendered output against the last deploy instruction sent
5. If anything changed (values, version, chart), emits a new **deploy instruction**

A deploy instruction is an immutable record telling a naisd agent to install or upgrade a specific feature version with specific helm values.

### What triggers reconciliation

- A new assignment is created
- A feature version changes
- Config values are updated
- Environment values change
- Periodic sweep (catches any drift)

## naisd

**naisd** is an agent that runs in each environment. It:

- Receives deploy instructions from Fasit (via Pub/Sub)
- Executes `helm install` or `helm upgrade`
- Reports success/failure status back to Fasit
- Self-upgrades when Fasit publishes a new naisd version

naisd is the only component that needs cluster access. Fasit itself never talks directly to Kubernetes.
