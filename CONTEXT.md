# Fasit

Multi-tenant feature management platform. Manages OCI Helm charts ("features") across environments, with deployment reconciliation and rollout orchestration via naisd agents.

## Language

**Feature**:
An OCI Helm chart with declared values (configs + computed) managed by Fasit and deployed to tenant environments.

**Computed value**:
A helm value whose output is derived from a Go template at render time, potentially referencing configs, environment values, or management values.

**Config value**:
A helm value supplied by operators per environment. May be marked `secret`.

**Secret taint**:
A computed value is "tainted" when its rendered output depends on at least one secret input (a secret config or a secret environment value). Tainted values are masked in the overview UI.

**Probe render**:
A second, disposable render of helm values where all secret inputs are replaced with a high-entropy sentinel. By comparing the probe output to a control render, the system identifies which computed values are tainted.

**Environment value**:
A key-value pair scoped to an environment within a tenant. May be marked `secret` at the storage layer.

**Management value**:
A key-value pair from the management environment for a tenant, available to all features via `.Management` in templates.

## Relationships

- A **Feature** has zero or more **Config values** and **Computed values**
- A **Computed value** may reference **Config values**, **Environment values**, and **Management values** in its template
- A **Computed value** acquires **Secret taint** when any referenced input is secret
- **Secret taint** is detected by comparing a **Probe render** against a control render

## Example dialogue

> **Dev:** "If a computed value uses `{{ .Env.token | b64enc }}`, is it tainted?"
> **Domain expert:** "Yes — if `token` is a secret environment value, the probe render substitutes a sentinel, so the b64enc output differs between control and probe. The diff flags `out` as tainted."

> **Dev:** "What if the template just checks `{{ if .Env.token }}static{{ end }}`?"
> **Domain expert:** "Not tainted — both renders produce `static` because the sentinel is truthy too. The output is identical."

## Flagged ambiguities

- "secret" was used to mean both "a config/env value declared as secret" and "a computed value whose output depends on a secret input." Resolved: the input property is **secret** (on config/env values); the derived property on computed values is **secret taint**.
