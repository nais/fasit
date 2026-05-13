# Probe-and-diff to detect secret taint in computed helm values

Computed helm values are Go templates that can reference secret inputs (secret configs, secret environment values). To prevent leaking secret-derived output in the overview UI, we need to know which computed values depend on secrets — but static analysis of arbitrary Go templates (with `with`, `range`, custom helpers like `mapOf`, Sprig functions) is fragile and incomplete.

We chose a **probe-and-diff** approach: render helm values twice — once with real inputs, once with secrets replaced by a high-entropy sentinel — and flag any computed key whose output differs. This catches all template patterns without parsing them, at the cost of a second render per page load.

## Considered options

- **Static template analysis**: walk the AST, flag keys whose templates reference secret-sourced variables. Rejected because Go templates rebind `.` in `with`/`range`, and custom helpers like `mapOf`/`eachOf` make static taint tracking unreliable. Would need to be updated every time a new helper is added.
- **Conservative "mask everything from `.Env`"**: any computed value touching `.Env`, `.Envs`, or `.Management` is masked. Simple but over-masks values like `{{ .Env.name }}` where `name` is never secret, eroding operator trust.
- **Probe-and-diff** (chosen): accurate for all template patterns, no coupling to template internals. Trades a second render for precision.

## Consequences

- **Non-deterministic template functions** (`now`, `randAlphaNum`, `uuidv4` from Sprig) cause both renders to differ regardless of secrets → false-positive taint. Mitigated by using a deterministic function map for both sides of the taint comparison.
- **Non-string secret configs** (int, bool, string_array) receive a string sentinel, which may cause the probe render to fail on type mismatch. On probe failure, all computed values are pessimistically masked. This is documented and tested as acceptable.
- **Scope**: only the environment config overview tab is masked. The helm values tab intentionally shows cleartext — operators who navigate there expect the full picture.
- **Single sentinel**: both env secrets and config secrets use the same high-entropy sentinel for the probe, simplifying collision analysis.
