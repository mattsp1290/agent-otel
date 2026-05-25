# Project Planning with Beads

## Agent Instructions

You are an expert software architect creating a comprehensive task breakdown. This task graph will be executed by AI agents working in parallel, coordinated through MCP Agent Mail with file reservations to prevent conflicts.

<quality_expectations>
Create a thorough, production-ready task graph. Include all necessary setup, implementation, testing, and documentation tasks. Go beyond the basics - consider edge cases, error handling, security considerations, and integration points. Each task should be specific enough for an agent to execute independently without ambiguity.
</quality_expectations>

## Project Information

### Links to Relevant Documentation

> Note: a sibling plan at `docs/prompts/extract-agent-otel.md` covers the
> mechanics of lifting `local-symphony/internal/obs/` into a shared module
> for advisor + symphony, with detailed §1–§14 sequencing. **This v2 plan
> expands scope** to (a) align with the OTel GenAI semantic conventions so
> spans flow natively into Datadog LLM Observability, (b) ship
> OpenLLMetry-compatible attribute emission, and (c) add lotel as a
> first-class local-dev sink via a dual-exporter convenience. It also
> drops the v1 plan's "wait a quarter" caveat — the user has explicitly
> opted to proceed and to ignore backwards compatibility.

Primary planning artifacts on disk (read directly, do not re-fetch):

- **Sibling plan**: `docs/prompts/extract-agent-otel.md` — §3 (pre-extraction
  work) and §11 (test parity) are still load-bearing; ignore §2 (the STRETCH
  caveat) and §9 (MCP byte-equal replay gate — no longer a constraint since
  backwards compat is out of scope for this round).
- `~/docs/eino/` — six HTML planning docs that lay out the shared-repos
  extraction plan. `agent-otel` is named in Phase 6 of
  `04-integration-plan.html` and detailed in `05-shared-repos-proposal.html`.
- `~/git/advisor` — first consumer. Key files:
  - `internal/telemetry/telemetry.go` — current bootstrap (OTel v1.43, semconv
    v1.40, OTLP gRPC to `localhost:4317` with no-op fallback)
  - `internal/telemetry/cli_init.go` — CLI/serve split with `InitServe`,
    `InitCLI`, `InitDisabled`, `ForceFlush`, `Shutdown`, and tighter CLI
    timeout behavior that migration beads must preserve or intentionally
    replace.
  - `internal/advisor/core/advise.go` — span + histogram emission site
    (`advisor.handle` span, `advisor.call.duration` histogram)
  - `internal/advisor/provider.go` — `Usage{InputTokens, OutputTokens, Available}` shape
    extracted from `eino.schema.Message.ResponseMeta.Usage`
- `~/git/local-symphony` — second consumer and source of the better implementation:
  - `internal/obs/spans.go` — 7 canonical span names
  - `internal/obs/instruments.go` — canonical metric constants for token,
    tool-call, fallback-engaged, latency, errors, queue, and run-state surfaces.
    Read the file for the current list rather than relying on a count in this
    prompt.
  - `internal/obs/cardinality.go` — documented allowlist
  - `internal/obs/slog.go` — `otelslog` bridge advisor lacks
  - `docs/otel-env-vars.md` — env-var precedence rules (currently
    "general wins"; agent-otel flips this to OTel-spec "per-signal wins")
  - `docker-compose.yml` + `docker/otel-collector-config.yaml` — local
    OTel-collector pipeline used by symphony's integration tests
  - `test/integration/lotelhelper/` — existing lotel CLI integration harness
    pattern; lift/adapt this instead of inventing a second lotel test harness.
- `~/git/lotel` — local OTLP collector that agent-otel treats as a
  first-class dev-loop sink. Receives OTLP/gRPC on `:4317` and OTLP/HTTP
  on `:4318`, stores to JSONL + DuckDB. Per `PARITY.md` it does **not**
  forward to external backends and is **not** `gen_ai.*`-aware today.
  **Cross-repo asks for lotel land in `~/git/lotel/.agents/requests/`** —
  file a markdown request there rather than attempting Rust changes.

External specs (cited; verify current official docs before locking an
implementation task to a specific semantic-convention shape):

- OpenTelemetry GenAI semantic conventions
  ([overview](https://opentelemetry.io/docs/specs/semconv/gen-ai/),
  [client spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/),
  [agent spans](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/),
  [metrics](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-metrics/),
  [events](https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-events/)).
  These conventions are still moving. The task graph must include a blocker
  documentation bead that pins the exact span names, event names, metric names,
  attributes, stability status, and `OTEL_SEMCONV_STABILITY_OPT_IN` behavior
  against the official docs current at implementation time.
- Datadog LLM Observability with OTel GenAI
  ([blog](https://www.datadoghq.com/blog/llm-otel-semantic-convention/),
  [docs](https://docs.datadoghq.com/llm_observability/instrumentation/otel_instrumentation/)).
  The Datadog mapping bead must verify the current native mapping for
  `gen_ai.request.model`, `gen_ai.usage.input_tokens`,
  `gen_ai.usage.output_tokens`, `gen_ai.provider.name`, and
  `gen_ai.operation.name` before implementation beads depend on those names.
- [OpenLLMetry (Traceloop)](https://github.com/traceloop/openllmetry) — most
  OTel-pure third-party convention in this planning context. agent-otel emits
  OTel-native attributes by default and optionally emits legacy
  OpenLLMetry-compatible attributes behind `WithOpenLLMetryCompat()`.
- [Greptime: How OTel traces LLM calls, agent reasoning, and MCP tools](https://www.greptime.com/blogs/2026-05-09-opentelemetry-genai-semantic-conventions) — useful end-to-end example of GenAI span hierarchies.

### Project Description

`agent-otel` is a shared Go library that gives any LLM-agent service (starting
with `advisor` and `local-symphony`) a single-import, batteries-included
OpenTelemetry layer aligned with the OTel GenAI semantic conventions.

It provides:

- **Unified bootstrap** — one `agent_otel.Init(ctx, Options) (*Providers, shutdown, error)`
  call wires traces, metrics, logs, and an `otelslog` bridge with spec-correct
  env-var precedence (per-signal `OTEL_EXPORTER_OTLP_*_ENDPOINT` overrides
  general `OTEL_EXPORTER_OTLP_ENDPOINT`, scheme-derived TLS, header auth, etc.)
  and a graceful no-op fallback when no collector is reachable.
- **Pre-built model-call instruments** emitting `gen_ai.*` attributes — model
  latency histogram, input/output token counters, errors-by-provider counter,
  fallback-engaged counter. Spans carry `gen_ai.request.model`,
  `gen_ai.usage.input_tokens`, `gen_ai.usage.output_tokens`,
  `gen_ai.provider.name`, `gen_ai.operation.name`, `gen_ai.system` so they
  route into Datadog LLM Observability automatically and are legible to
  OpenLLMetry-aware backends.
- **Machine-enforced cardinality allowlist** — runtime + lint-time checks
  that a metric label set never exceeds the budgeted dimensions (lifting
  symphony's `cardinality.go` from documentation-only enforcement into both
  a test helper AND an opt-in runtime gate). The task graph must first define
  the runtime enforcement API; do not assume raw OTel instruments can be
  intercepted after callers pass arbitrary `metric.WithAttributes(...)`.
- **Dual-exporter convenience** — a `WithDevSink(lotelEndpoint)` option that
  sends telemetry both to the operator-configured OTLP backend (Datadog
  Agent, hosted OTel collector, etc.) and to a local `lotel` instance for
  test-artifact debugging, with the local sink failing open so production
  posture is never compromised by a missing dev collector.
- **Normalized usage recording** — helper APIs record
  `Usage{InputTokens, OutputTokens, Available}` values supplied by callers.
  Provider-specific Eino usage extraction and normalization belongs in the
  shared Eino/provider layer, because different adapters surface usage through
  different fields. An optional `agentotel/eino` adapter may be planned only if
  duplication remains after the Eino/provider extraction; the core bootstrap
  path must not depend on Eino.

Out of scope:

- Any Rust changes to `lotel` (file requests in `~/git/lotel/.agents/requests/` instead).
- Framework-specific instrumentation. `agent-otel` does not construct Eino
  providers, tools, or graphs; it exposes generic span/instrument helpers.
  Eino examples may live in docs or an optional adapter package.
- Multi-tenant / SaaS-style auth.
- **Backwards compatibility** — both consumers cut over to agent-otel
  semantics in the same set of migration PRs. Existing histograms, span
  names, and the MCP byte-equal replay gate (advisor `advisor-d96`,
  ADR-0001 §16–23) can be replaced rather than preserved. §3b of the
  sibling plan still applies in that ADR-0001 needs a superseding ADR,
  but the corpus regeneration is a one-time cost rather than a guarded
  migration.

### Technical Stack

- **Language**: Go 1.25.5 (matches the eino plan's Phase 0 alignment).
- **Core OTel deps**:
  - `go.opentelemetry.io/otel` v1.43
  - `go.opentelemetry.io/otel/sdk` (trace, metric, log providers)
  - `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc`
  - `go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc`
  - `go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc`
  - HTTP variants of all three (`otlptracehttp`, `otlpmetrichttp`, `otlploghttp`)
    for environments that block gRPC (and for the Datadog-Agent OTLP/HTTP path).
  - `go.opentelemetry.io/contrib/bridges/otelslog` for stdlib `log/slog` bridging.
  - `go.opentelemetry.io/otel/semconv/v1.40.0` as the floor for resource
    attributes. The task graph must include local string constants and literal
    tests for all `gen_ai.*` keys unless implementation-time research proves
    the Go semconv package exposes the exact needed experimental constants.
- **Optional Eino adapter only**: if included, isolate
  `github.com/cloudwego/eino` v0.8.13 behind a sub-package. Do not make core
  `agent-otel` depend on Eino, and do not assign provider-specific usage
  normalization to this repo.
- **Testing**:
  - Go stdlib `testing` + `github.com/stretchr/testify` for assertions.
  - In-process OTLP test receiver (custom or `otel-collector-testbed`-style)
    to assert span/metric/log emission shape without external processes.
  - Integration suite that spins up a real `lotel` instance via
    the lifted/adapted `~/git/local-symphony/test/integration/lotelhelper`
    pattern and asserts on lotel's DuckDB queries. Use unique `service.name`
    values, `ForceFlush` before `Shutdown`, `lotel-cli ingest`, query by
    service/since, and an `integration` build tag.
- **Build/CI**:
  - `golangci-lint` v1.61+ (matches existing repo configs).
  - GitHub Actions matrix: `go test -race`, lint, integration tests gated
    behind a label or nightly schedule (lotel startup is non-trivial).
  - `goreleaser` not needed (library, not binary).

### Specific Requirements

**Integrations (must-have)**:

- **OTel-native** — anything that speaks OTLP/gRPC or OTLP/HTTP works out
  of the box; no Datadog- or vendor-specific code required to use it.
- **Datadog LLM Observability** — verified that spans emitted with the
  `gen_ai.*` attribute set above map cleanly to Datadog's native LLM
  Observability schema. Include a `WithDatadogPreset()` option that sets
  sane defaults (HTTP exporter, `DD-API-KEY` header injection from env,
  recommended sampling, semconv stability opt-in) but is not required to
  use Datadog. Include an integration test that asserts the on-wire
  attribute names Datadog cares about.
- **OpenLLMetry / Traceloop** — span and event shape must be consumable by
  OpenLLMetry-aware backends without translation. Where OpenLLMetry's
  attributes overlap with OTel GenAI (e.g., `traceloop.entity.name` vs
  `gen_ai.agent.name`), emit the OTel-native attribute and document the
  mapping. Include a `WithOpenLLMetryCompat()` option that additionally
  emits the legacy `llm.*` / `traceloop.*` attributes for older consumers.

**Other notes**:

- **Backwards compatibility**: ignored. Both consumers cut over in lockstep.
  ADR-0001's `service.version` literal clause is superseded by a new ADR
  in the advisor repo (sibling plan §3b), and the MCP byte-equal replay
  corpus is regenerated once as a one-time cost rather than preserved.
- **Performance / cardinality**: enforce cardinality at runtime with a
  low-overhead allowlist check (default: log-and-drop the prohibited label,
  not a panic); document the budget per-instrument. The graph must include a
  blocking API-design bead that decides whether the public surface is wrapper
  recorders such as `RecordModelLatency(ctx, value, Labels)` /
  `AddFallbackEngaged(ctx, Labels)`, how unknown metric names behave, and
  whether prohibited-label logging is once-per-key or every call. Default
  exporter is async-batch.
- **Privacy / prompt capture**: opt-in only, off by default. Provide a hook
  interface (`PromptRedactor`) so callers can install regex/structured
  scrubbers before payloads are added as events. Document the redaction
  contract: redactor runs **before** the event is attached to the span,
  and the unredacted payload is never logged.
- **Cross-repo coordination**: any lotel-side change needed (e.g., a
  `gen_ai.*`-aware query view, a forwarder to Datadog, a stable
  attribute index) is filed as a markdown request in
  `~/git/lotel/.agents/requests/` rather than attempted in lotel's Rust
  source. Track requests in this repo's task graph so they don't get lost.

---

## Your Task

Analyze this project and create a comprehensive **Beads task graph** using the `bd` CLI. Beads provides dependency-aware, conflict-free task management for multi-agent execution.

---

<critical_constraint>
Your ONLY output is a bash shell script printed to stdout. Do NOT write files,
run the generated script, or implement product code yourself. Do NOT use
`bd add` — the correct command to create a bead is `bd create`. Use
`bd dep add` for dependencies.
</critical_constraint>

## Output Format

Generate a shell script that creates the full task graph. The script should:

1. **Initialize Beads** (if not already initialized)
2. **Create all beads** with appropriate priorities
3. **Establish dependencies** between beads
4. **Add labels** for phase grouping

### Example Output

```bash
#!/bin/bash
# Project: agent-otel
# Generated: 2026-05-25

set -e

# Initialize beads if needed
if [ ! -d ".beads" ]; then
    bd init
fi

echo "Creating project beads..."

# ========================================
# Phase 1: Project Setup & Infrastructure
# ========================================

SETUP_REPO=$(bd create "Initialize Go module github.com/mattsp1290/agent-otel with go 1.25.5" -p 0 --labels setup --description "Create the initial Go module skeleton and pin Go 1.25.5." --acceptance "go test ./... runs against the empty skeleton; go.mod declares github.com/mattsp1290/agent-otel and Go 1.25.5." --silent)

SETUP_LINT=$(bd create "Configure golangci-lint v1.61+ matching advisor/symphony configs" -p 1 --labels setup --description "Add lint configuration derived from existing advisor/local-symphony settings." --acceptance "golangci-lint run ./... passes and documented deviations from advisor/local-symphony configs are justified." --silent)
bd dep add $SETUP_LINT $SETUP_REPO

SETUP_CI=$(bd create "Add GitHub Actions: go test -race, lint, nightly lotel integration tests" -p 1 --labels setup --description "Add CI workflows for race tests, lint, and gated lotel integration coverage." --acceptance "Workflow YAML includes go test -race ./..., golangci-lint, and integration-tagged lotel job gated behind schedule/label/manual trigger." --silent)
bd dep add $SETUP_CI $SETUP_LINT

# ... continue for all phases ...

echo ""
echo "Bead graph created! View with:"
echo "  bd ready              # List unblocked tasks"
```

---

## Bead Creation Guidelines

### Priority Levels
- `-p 0` = Critical (blocking other work)
- `-p 1` = High (important but not blocking)
- `-p 2` = Medium (standard work)
- `-p 3` = Low (nice to have)

### Labels (Phase Grouping)
Use `--labels` (or `-l`) to group beads by phase. Multiple labels are
comma-separated:
- `setup` - Project initialization
- `core` - Core bootstrap, env-var precedence, providers
- `instruments` - Pre-built model-call instruments
- `genai-semconv` - OTel GenAI attribute emission
- `datadog` - Datadog preset + integration assertions
- `openllmetry` - OpenLLMetry compatibility shim
- `lotel-devsink` - Dual-exporter convenience
- `cardinality` - Allowlist enforcement (test + runtime)
- `usage` - Normalized usage recording and optional adapter boundaries
- `privacy` - PromptRedactor hook
- `migrate-advisor` - Advisor cutover PR
- `migrate-symphony` - Symphony cutover PR
- `pre-extraction` - Sibling plan §3a/§3b/§3c work
- `lotel-request` - Markdown asks filed into ~/git/lotel/.agents/requests/
- `testing` - Test coverage
- `docs` - Documentation

### Dependency Rules
1. Never create cycles
2. Every bead should have a clear dependency chain back to setup tasks
3. Use `bd dep add CHILD PARENT` (child depends on parent completing first)
4. Parallel work should share a common ancestor, not depend on each other

### Task Granularity
- Each bead should be completable in **under 750 lines of code**
- Tasks should be atomic enough for one agent to complete without coordination
- If a task requires multiple file areas, consider splitting by file area
- Every `bd create` command must include `--description` and `--acceptance`.
  The description must name the expected file reservation surface and the
  acceptance criteria must name the verification command or concrete review
  artifact.

### Required Blocker Design Beads

Before implementation beads for core exporters, GenAI attributes, Datadog,
OpenLLMetry, lotel, cardinality, or consumer migrations are unblocked, create
documentation/design beads for:

- `docs/bootstrap-design.md` — per-signal endpoint/header/insecure/protocol
  resolution, OTLP/gRPC vs OTLP/HTTP URL handling, Datadog header injection,
  dev-sink SDK composition, primary/dev fail-open rules, and ForceFlush /
  shutdown order.
- `docs/genai-mapping.md` — current official OTel GenAI span, event, metric,
  attribute, stability, and opt-in behavior; exact local string constants when
  Go semconv lacks constants; choose and justify counter vs histogram shape for
  token usage; serialized OTLP assertions required by tests.
- `docs/telemetry-migration-map.md` — source-to-target mapping for advisor and
  symphony call sites: old span/metric names, old attrs, new `agent-otel`
  helper/API, new span/metric names, new attrs, retained product-specific attrs,
  dropped attrs, and required tests. Include advisor `advisor.provider`,
  `advisor.model`, `advisor.input_tokens`, `advisor.output_tokens`,
  `advisor.usage_available`, `advisor.latency_ms`, `advisor.entrypoint`, and
  `error.type`; include symphony `model.name`, `direction`, `model.from`,
  `model.to`, `request_kind`, `error.kind`, tool-call attrs, fallback attrs,
  and Ollama request spans.
- `docs/datadog.md` — exact Datadog LLM Observability attribute expectations,
  preset behavior, required headers, sample values, and on-wire tests.
- `docs/openllmetry.md` — OTel-native attributes plus optional legacy
  `llm.*` / `traceloop.*` compatibility attributes and tests.
- `docs/cardinality-runtime.md` — runtime validation API, allowed/prohibited
  label behavior, unknown metric behavior, log-and-drop policy, and lint/test
  helpers.
- `docs/usage-boundary.md` — normalized
  `Usage{InputTokens, OutputTokens, Available}` contract, `Available=false`
  behavior, and the boundary with `eino-providers`/framework-specific
  extraction. Include examples for advisor's single-shot `Provider.Advise`
  path and symphony's Eino graph path around model/tool/collect steps, but keep
  those examples out of the core implementation contract.

Implementation beads must depend on the relevant design bead instead of
inventing these decisions inline.

---

## File Reservation Planning

For each major work area, note the file patterns that will need exclusive reservation:

```bash
# Core bootstrap:     bootstrap.go, config.go, cleanup.go, *_test.go for these
# Instruments:        instruments.go, instruments_test.go
# GenAI semconv:      spans.go, genai_attrs.go, spans_test.go
# Datadog preset:     presets/datadog.go, presets/datadog_test.go
# OpenLLMetry compat: presets/openllmetry.go, presets/openllmetry_test.go
# Lotel dev sink:     devsink/lotel.go, devsink/lotel_test.go
# Cardinality:        cardinality.go, cardinality_test.go
# Usage helpers:      usage.go, usage_test.go, optional eino/usage.go if justified
# Privacy:            redact.go, redact_test.go
# Cross-repo:         ~/git/lotel/.agents/requests/<request>.md (one file per ask)
# Advisor migration:  ~/git/advisor/internal/telemetry/**,
#                     ~/git/advisor/internal/advisor/core/**,
#                     ~/git/advisor/internal/mcp/**,
#                     ~/git/advisor/internal/cli/**,
#                     ~/git/advisor/integration/**,
#                     ~/git/advisor/docs/adr/**
# Symphony migration: ~/git/local-symphony/internal/obs/**,
#                     ~/git/local-symphony/internal/worker/**,
#                     ~/git/local-symphony/test/integration/**,
#                     related docs/dashboard/test files
```

This helps agents claim appropriate file surfaces when they start work.

---

## Context Documentation

Create beads that produce context docs under `docs/` for agents to reference.
The generated bash script must not write these docs itself; it must create
documentation beads with file reservations, dependencies, descriptions, and
acceptance criteria. Required docs include:

- `docs/bootstrap-design.md`
- `docs/genai-mapping.md`
- `docs/telemetry-migration-map.md`
- `docs/datadog.md`
- `docs/openllmetry.md`
- `docs/cardinality-runtime.md`
- `docs/usage-boundary.md`
- `docs/redaction.md`
- `docs/env-vars.md`
- Any filed `~/git/lotel/.agents/requests/` markdown should be copied or
  symlinked from a documentation bead so the task graph references it.

---

## Verification Steps

The generated script must print these operator instructions at the end:

1. **Run it**: `chmod +x setup-beads.sh && ./setup-beads.sh`
2. **Check ready work**: `bd ready` should show initial setup tasks plus the
   three pre-extraction items from the sibling plan §3 (symphony semconv bump,
   advisor ADR-0009, env-var precedence reconciliation) and the blocker
   design-doc beads. Those are independent and can run in parallel with
   agent-otel skeleton work where dependencies allow.

---

## Completeness Checklist

Ensure your task graph includes:

- [ ] Repo init, Go module, lint, CI
- [ ] Pre-extraction work (sibling plan §3a/§3b/§3c) as parallel tracks with `pre-extraction` label
- [ ] Blocker docs: bootstrap design, GenAI mapping, telemetry migration map, Datadog mapping, OpenLLMetry mapping, cardinality runtime design, usage boundary, redaction, env vars
- [ ] Core bootstrap (`Init`, `Options`, `Providers`, OTel-spec env-var precedence)
- [ ] Pre-built instruments (`ModelLatency`, `UsageInputTokens`, `UsageOutputTokens`, `ErrorsByProvider`, `FallbackEngaged`) with `gen_ai.*` attribute emission
- [ ] OTel GenAI span name + local attribute constants (`gen_ai.request.model`, `gen_ai.usage.*`, `gen_ai.provider.name`, `gen_ai.operation.name`, `gen_ai.system`) with literal-name tests
- [ ] Datadog preset (`WithDatadogPreset()`) + integration test asserting on-wire attribute shape
- [ ] OpenLLMetry compat shim (`WithOpenLLMetryCompat()`) + attribute-mapping doc
- [ ] Lotel dev-sink (`WithDevSink(endpoint)`) with fail-open semantics + integration test against a real lotel instance
- [ ] Lotel request-directory setup/verification bead before any request-file bead writes `~/git/lotel/.agents/requests/`
- [ ] Cardinality allowlist (test helper + opt-in runtime gate with log-and-drop default) using the API chosen in `docs/cardinality-runtime.md`
- [ ] `otelslog` bridge wiring (lifted from symphony, parameterized)
- [ ] Normalized usage recording from `Usage{InputTokens, OutputTokens, Available}`; optional Eino adapter only if justified by `docs/usage-boundary.md`
- [ ] `PromptRedactor` hook for opt-in payload capture
- [ ] In-process OTLP test receiver harness
- [ ] Lotel integration harness lifted/adapted from `~/git/local-symphony/test/integration/lotelhelper`
- [ ] Advisor migration PR covering telemetry bootstrap, CLI/serve split, core span/metric emission, CLI/MCP parity, integration tests, and ADR updates
- [ ] Symphony migration PR (sibling plan §13 PR5)
- [ ] Documentation: env-var precedence, GenAI attribute table, Datadog mapping, OpenLLMetry mapping, cardinality budget, redactor contract
- [ ] Any lotel asks captured as `lotel-request` beads with the markdown file path in the description
- [ ] Clear dependency chains with no cycles
