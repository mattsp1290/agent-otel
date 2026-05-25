# Extract `agent-otel` from advisor + local-symphony

> **STRETCH, NOT MANDATORY — READ THIS FIRST**
>
> `agent-otel` is the third of four proposed shared modules and is explicitly tagged
> STRETCH in the research at `~/docs/eino/05-shared-repos-proposal.html#agent-otel`.
> Of the four extractions, this one has by far the largest impedance mismatch
> between its two would-be consumers:
>
> - **Code-size mismatch.** Advisor's telemetry package is ~156 lines
>   (`~/git/advisor/internal/telemetry/telemetry.go`); symphony's `obs` package is
>   ~1,400 lines across nine files (`~/git/local-symphony/internal/obs/`). This is
>   not a verbatim lift — it is "lift the symphony shape and migrate advisor onto
>   it."
> - **semconv version mismatch.** Advisor pins `go.opentelemetry.io/otel/semconv/v1.40.0`
>   (`telemetry.go:17`, `cli_init.go:15`). Symphony pins
>   `go.opentelemetry.io/otel/semconv/v1.26.0` (`otel.go:24`). The two cannot
>   coexist inside a single shared module.
> - **service.version literal mismatch.** Advisor uses
>   `attribute.String("service.version", version)` (`telemetry.go:66`,
>   `cli_init.go:186`) and ADR-0001 (`~/git/advisor/docs/adr/0001-telemetry-init-and-shutdown.md`,
>   clauses 16–23) makes that literal **load-bearing** for a byte-equal MCP replay
>   gate (`advisor-d96`). Symphony uses `semconv.ServiceVersion(version)` +
>   `semconv.DeploymentEnvironment(env)` (`otel.go:248–250`).
> - **OTel SDK version skew.** Advisor is on `v1.43.0` + Go `1.25.5`. Symphony is on
>   `v1.41.0` + Go `1.25.0`.
> - **Log-signal coverage.** Symphony emits OTel log records through the otelslog
>   bridge (`slog.go`); advisor does not.
>
> Neither attribute shape is wrong; each is load-bearing in its own project. A
> shared module **must** unify these before either consumer migrates.
>
> **Recommendation:** Let both consumers remain on their own copies for **at
> least one quarter of in-flight production use** before scheduling this
> extraction. `codex-auth-go` and `eino-providers` are the priorities; this can
> lag. If forced to schedule, treat the three pre-extraction items in §3 as the
> real cost, not the eventual lift.

---

## 1. Goals and non-goals

`agent-otel` is a small Go module that gives any agent-flavored Go service a
one-call boot path for OpenTelemetry: trace + metric + (optional) log providers,
OTLP gRPC exporters, no-op fallback on dial failure, and a curated bundle of
**model-call** instruments that advisor and symphony both want.

**Goals.**

- Provide a single `Init(ctx, Options) (*Providers, shutdown, error)` that builds
  trace + meter + logger providers, OTLP-gRPC exporters, a resource describing
  the service, and a pre-built `*Instruments` bundle.
- Encode the env-var precedence rules currently scattered between advisor and
  symphony so future projects do not re-derive them.
- Encode a **cardinality budget** at instrument-record time so a runaway label
  cannot blow up the meter.
- Provide canonical span / metric / attribute-key constants for the LLM
  model-call surface (`provider`, `model`, `usage.input_tokens`,
  `usage.output_tokens`, `usage.available`).
- Provide a tested coordinated shutdown that drains logs, then metrics, then
  traces.

**Non-goals.**

- **Model-specific instrumentation.** Symphony today hard-codes an
  `OllamaLatency` histogram and `MetricOllamaErrors` counter
  (`instruments.go:27–28`, `212–226`). The replacement in `agent-otel` is a
  generic `ModelLatency` plus `ErrorsByProvider`; per-engine specialisation
  stays in the caller.
- **Tracing for non-Anthropic / non-OpenAI / non-Google providers.** Out of
  initial scope. Callers wanting Bedrock / Mistral / etc. add their own
  instrumentation against the exposed `*Instruments` and span-key constants.
- **Log shipping infrastructure.** `agent-otel` emits OTLP signals; an OTel
  Collector ships them. Collector configuration, retry, queueing, etc., are
  out of scope.
- **Project-specific labels.** Symphony's `model.from` / `model.to` (used only
  for the fallback-engaged counter, `cardinality.go:30`) and advisor's
  advisor-call histogram (`telemetry.go:141–147`) both stay project-side.

---

## 2. Why this is STRETCH (and what that means for sequencing)

The shared-repos research has been corrected (`05-shared-repos-proposal.html`
§3, paragraphs labeled "Real impedance mismatch (read before scheduling
this)") to flag every concrete obstacle below; this section restates them for
the planning context.

- **156 vs 1,400 lines.** A naive symbol-by-symbol lift would not work in either
  direction. Adopting symphony's shape and migrating advisor onto it is the
  only feasible path.
- **semconv `v1.40.0` vs `v1.26.0`.** Resource attribute keys overlap but are
  not bit-for-bit identical (some helper functions return different
  `attribute.Key` values across versions). Picking the newer (`v1.40.0`)
  matches advisor today and is forward-aligned with the upstream SDK.
- **Advisor's MCP byte-equal replay gate.** ADR-0001 pins the
  `attribute.String("service.version", version)` literal explicitly because the
  helper-based shape would change the on-wire bytes. The gate (`advisor-d96`)
  treats any drift as a CI failure. This is the single most expensive
  pre-extraction item — see §3b and §9.
- **Symphony's typed `ConfigValidationError`** (`config.go:42–65`) fails fast
  on operator typos and supports `errors.Is` against `strconv.ErrSyntax`.
  Advisor has no equivalent and silently dials a malformed endpoint until the
  per-RPC timeout fires. This is a real behaviour change advisor adopts.
- **Symphony's log-signal pipeline** (`slog.go`, `otelslog.NewHandler`) does
  not exist in advisor. Advisor currently does not consume `LoggerProvider`
  at all. Adoption is purely additive on the advisor side, but it is a new
  shape advisor's callers must opt into.

The reasonable consequence: **leave both consumers on their own copies for
≥1 quarter of stable use** while the upstream OTel SDK semantics stabilise
and while the higher-priority modules (`codex-auth-go`, `eino-providers`)
land. Revisit when (a) the `Usage*` token-accounting work in symphony Phase 4
(`~/docs/eino/04-integration-plan.html`, Phase 4) is in production, and (b)
advisor's ADR-0001 successor (§3b) is signed off and the replay corpus is
regenerated.

---

## 3. Pre-extraction work (the real cost)

Three changes must land **before** any extraction work begins. Each is 1–3
days of careful work and each can be done independently of the others, so
they can land in parallel.

### 3a. Bump symphony's `semconv` to `v1.40.0`

- File: `~/git/local-symphony/internal/obs/otel.go` line 24
  (`semconv "go.opentelemetry.io/otel/semconv/v1.26.0"`).
- Update to `semconv/v1.40.0` and verify (a) `semconv.ServiceName`,
  `semconv.ServiceVersion`, `semconv.DeploymentEnvironment` still resolve
  with identical attribute keys (`service.name`, `service.version`,
  `deployment.environment`); (b) the `SchemaURL` constant changes from
  `1.26.0` to `1.40.0` and operators see the new URL in resource attributes;
  (c) no existing Grafana / Datadog dashboard rule keys off `SchemaURL`.
- This is an independent symphony PR with no agent-otel dependency. If it
  reveals a downstream consumer pinning to schema URL `1.26.0`, that
  consumer is fixed before extraction begins.

### 3b. Supersede ADR-0001's `service.version` clause in advisor

- File: `~/git/advisor/docs/adr/0001-telemetry-init-and-shutdown.md`,
  clauses 16–23. The current rule pins
  `attribute.String("service.version", version)` for byte-equal MCP replay.
- Write a new ADR (next free number — likely **ADR-0009**) that:
  - Cites ADR-0001 §16–23 and explicitly supersedes that clause.
  - Permits `semconv.ServiceVersion(version)` in place of the raw literal.
  - States the new replay-corpus generation procedure (see §9).
  - Names the migration window and the owner.
- Regenerate the byte-equal MCP replay corpus and update the gate's golden
  file. The exact path is named in ADR-0001 — the planning task pulls the
  current path at the time of the ADR-0009 PR rather than baking it into
  this document, because the corpus location may have moved.
- Get sign-off from the advisor maintainers before any `agent-otel`
  consumer-migration PR opens.

### 3c. Reconcile env-var precedence

- Advisor: does not enforce per-signal beats global beats default; the
  `defaultEndpoint = "localhost:4317"` constant (`telemetry.go:22`) is
  applied unconditionally.
- Symphony: applies a "general wins over per-signal" rule (see
  `~/git/local-symphony/docs/otel-env-vars.md` lines 12–20) — this is
  itself a deviation from the OTel spec, which says per-signal beats
  general.
- Decision needed before extraction: pick **one** unified rule. The
  recommendation is to adopt the **OTel-spec rule** (per-signal beats
  general beats default), which is what `agent-otel` should ship in §8.
  That means symphony also flips its current "general wins" behaviour as
  part of extraction. Operators with the documented combined-set behaviour
  get a one-release deprecation warning before flip.
- This contradicts the symphony status quo documented in
  `otel-env-vars.md`; that doc is rewritten when the rule flips.

Each of 3a / 3b / 3c is independent and small; together they are the gate
on starting §4. None of them require any work in `agent-otel` itself.

---

## 4. Module layout

File-by-file proposal for the `agent-otel` Go module:

- **`bootstrap.go`** — public `Init(ctx context.Context, opts Options)
  (*Providers, func(context.Context) error, error)`. Wires OTLP-gRPC
  exporters; on dial failure within `opts.DialTimeout`, falls back to
  no-op providers. Calls `otel.SetTracerProvider` /
  `otel.SetMeterProvider` / `logglobal.SetLoggerProvider` unless
  `opts.SkipGlobalInstall` is set. Lifted from `otel.go:141–214` (the
  `initWithFactories` flow), simplified for the post-3c precedence rule.
- **`config.go`** — `LoadConfigFromEnv()`, `ConfigValidationError`,
  `normalizeEndpoint`, `validateEndpoint`, `parseHeaders`, `envBool`.
  Lifted from `~/git/local-symphony/internal/obs/config.go` lines 42–622
  with one rename pass to drop `SYMPHONY_*` prefixes — see §6.
- **`cardinality.go`** — `AllowedKeys`, `ProhibitedKeys`,
  `ProhibitedPrefixes`, `CheckAllowedLabels`, `IsProhibitedKey`. Lifted
  verbatim from `~/git/local-symphony/internal/obs/cardinality.go` with
  the symphony-specific metric-name map (`allowedLabels`) replaced by a
  module-default map covering the generic `ModelLatency`,
  `UsageInputTokens`, `UsageOutputTokens`, `ErrorsByProvider`,
  `FallbackEngaged` entries. Caller projects extend the map via a
  registration API (see §7).
- **`instruments.go`** — common pre-built model-call instruments:
  - `ModelLatency` (renamed from `MetricOllamaLatency`,
    `instruments.go:27,212–218`) — `Float64Histogram`, unit `s`.
  - `UsageInputTokens`, `UsageOutputTokens` — `Int64Histogram`, unit
    `{token}`. New in `agent-otel`; symphony Phase 4 will populate.
  - `ErrorsByProvider` — `Int64Counter`, label `provider`,
    `error.kind`. Generalisation of `MetricOllamaErrors`.
  - `FallbackEngaged` — `Int64Counter`, label `provider.from`,
    `provider.to`. Lifted from `MetricFallbackEngaged`
    (`instruments.go:24,188–194`) but generic across providers.
- **`spans.go`** — span name + attribute-key constants:
  `SpanModelRequest`, plus key constants `AttrProvider` (= `provider`),
  `AttrModel` (= `model`), `AttrUsageInputTokens` (=
  `usage.input_tokens`), `AttrUsageOutputTokens` (=
  `usage.output_tokens`), `AttrUsageAvailable` (= `usage.available`).
  Helper setters for span attribute groups go here; the existing
  symphony file `spans.go` is only constants today, so this file expands
  the surface area meaningfully.
- **`slogbridge.go`** — `SlogHandler`, `SlogHandlerOptions`,
  `traceContextHandler`, `teeHandler`. Lifted from
  `~/git/local-symphony/internal/obs/slog.go` lines 1–237 with the
  `DefaultServiceName` constant in `otelslog.NewHandler` (line 68)
  replaced by `opts.ServiceName` so callers do not silently log under
  `"symphony"`.
- **`cleanup.go`** — extracted coordinated-shutdown logic. Today
  symphony's shutdown lives inside `otel.go:111–122` (the `Provider.Shutdown`
  method); the extraction moves the per-component closure ordering
  (logs → metrics → traces) into a separate file so the test parity
  in `cleanup_test.go` can lift cleanly. Advisor's
  `cli_init.go:243–264` flush-then-shutdown sequence merges here too.

---

## 5. What stays in each consumer

### Advisor

- The `metric.Int64Histogram` named `advisor.call.duration`
  (`telemetry.go:141–147`) stays advisor-owned. It is specific to the
  advisor product surface — `agent-otel` does not ship product-named
  instruments.
- `cli_init.go` shrinks from ~280 lines to a ~10–20 line caller that
  builds an `agent-otel` `Options`, calls `agent_otel.Init`, and
  registers advisor's product histogram against the returned
  `*Providers.Meter`.
- The `Shutdown` struct wrapper in `cli_init.go:44–66` is retained
  if advisor wants to keep the `ForceFlush` / `Shutdown` two-method
  interface as a thin wrapper around the `agent-otel` shutdown
  closure — that's a 30-line decorator, no functional change. ADR-0004
  references this shape and stays valid.

### Symphony

- All the orchestrator-specific instruments in `instruments.go` that
  are **not** in the agent-otel generic set stay symphony-side:
  `PollDuration`, `DispatchLatency`, `RunsActive`, `Retries`,
  `QueueDepth`, `TokensAggregate` (vs. agent-otel's `UsageInput/
  OutputTokens`), `TokensPerTurn`, `ToolCalls`, `ToolCallsMalformed`,
  `DroppedEvents`, `FailedFinalize`, `PersistenceWrite`.
- The `MetricFallbackEngaged` counter migrates to agent-otel's
  `FallbackEngaged` because circuit-breaker patterns are generic to
  model-call agent code; symphony's call sites then move to the new
  generic counter and update their label keys from `model.from /
  model.to` to `provider.from / provider.to`.
- Symphony retains its `model.from` / `model.to` *span* attributes
  per `spans.go:20–24` — those are span-only and do not flow through
  the metric cardinality budget.

---

## 6. Public API surface (sketch)

```go
type Options struct {
    ServiceName       string
    Version           string
    DeploymentEnv     string
    Endpoint          string
    Insecure          bool
    Headers           map[string]string
    Logger            *slog.Logger
    CardinalityBudget int
    DialTimeout       time.Duration
    BatchTimeout      time.Duration
    ExportTimeout     time.Duration
    SkipGlobalInstall bool
    EnableLogs        bool
}
```

```go
type Providers struct {
    TracerProvider trace.TracerProvider
    MeterProvider  metric.MeterProvider
    LoggerProvider log.LoggerProvider
    Tracer         trace.Tracer
    Meter          metric.Meter
    Instruments    *Instruments
}

type Instruments struct {
    ModelLatency      metric.Float64Histogram
    UsageInputTokens  metric.Int64Histogram
    UsageOutputTokens metric.Int64Histogram
    ErrorsByProvider  metric.Int64Counter
    FallbackEngaged   metric.Int64Counter
}

func Init(ctx context.Context, opts Options) (*Providers, func(context.Context) error, error)
```

Env-var names drop the `SYMPHONY_*` prefix from symphony's `config.go`
(lines 98–127) and adopt the neutral `AGENT_OTEL_*` form for the
project-specific ones (`AGENT_OTEL_DISABLED`, `AGENT_OTEL_SERVICE_NAME`,
`AGENT_OTEL_SERVICE_VERSION`, `AGENT_OTEL_DEPLOYMENT_ENVIRONMENT`). The
standard `OTEL_*` env vars are honoured unchanged.

---

## 7. Cardinality budget

Symphony's `cardinality.go` enforces a per-metric label allowlist at
**test time**, not at instrument-record time
(`cardinality.go:96–108`: "This function is intended for use in tests
that validate emit sites. Production emit code does not call it on the
hot path.").

`agent-otel` keeps the same chokepoint design but:

- Exposes a `RegisterAllowedLabels(metricName string, keys ...string)`
  function so caller projects extend the default map at startup. The
  default map covers the five generic instruments listed in §4.
- The `CheckAllowedLabels` helper remains test-only — production code
  paths still pay zero hot-path cost. Caller projects keep a
  test like `~/git/local-symphony/internal/obs/cardinality_test.go`
  that walks every emit site and validates against the merged map.
- Documents the prohibited prefix set verbatim from
  `cardinality.go:39–55` (`issue.id`, `session.id`, `thread.id`,
  `turn.id`, `run_attempt.id`, `tool.input/*`). Caller projects can
  extend the prohibition list but cannot shrink it.

The most subtle design point: the budget enforcement is **at attribute
addition time**, not at provider-init time. A label key that is silently
added to a metric record without being in the allowlist passes through
unless the test harness catches it. The pre-extraction work in §3 does
**not** include making this a hard runtime check; making it a runtime
check is a deferred open question (§14).

---

## 8. Env-var precedence

After §3c lands, the unified rule (matching the OTel SDK spec) is:

1. **Per-signal env var.** `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT`,
   `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT`, `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT`.
2. **Global env var.** `OTEL_EXPORTER_OTLP_ENDPOINT`.
3. **Default.** `localhost:4317` (matches `defaultEndpoint`,
   `telemetry.go:22`, and `DefaultEndpoint`, `config.go:72`).

The same precedence applies for headers (`OTEL_EXPORTER_OTLP_*_HEADERS`)
and the insecure flag (`OTEL_EXPORTER_OTLP_*_INSECURE`).

In `bootstrap.go`, the exporter factories pass `WithEndpoint(cfg.Endpoint)`
**only** when the caller's `opts.Endpoint` is non-empty AND not derived
from the in-process default — same `EndpointExplicit` discrimination
symphony uses today (`config.go:158–187`). When neither the caller nor
the global env var supplied an endpoint, no `WithEndpoint` option is
appended and the OTel SDK consumes the per-signal env vars directly.

Worked examples for operators belong in a new
`agent-otel/docs/env-vars.md`, modeled on
`~/git/local-symphony/docs/otel-env-vars.md` but rewritten to reflect
the per-signal-first rule. That doc and the precedence helper code in
`config.go` MUST be updated in lockstep — copy the same maintenance
note symphony uses today
(`~/git/local-symphony/docs/otel-env-vars.md:8`).

---

## 9. MCP replay-gate caution

The byte-equal MCP replay parity gate (advisor's `advisor-d96`) is the
**single most important constraint** in this extraction. If
`agent-otel` ships a single byte that differs from the legacy advisor
`telemetry.Init` output, the gate fails and the advisor migration
blocks indefinitely.

Documented contract for the extraction PR:

- **Code paths that produce on-wire bytes** that the gate compares:
  - The resource attribute set: schema URL, `service.name`,
    `service.version` literal vs helper, `deployment.environment`
    (new), any process / host attributes auto-detected by
    `resource.New(resource.WithFromEnv(), WithProcess(), WithHost())`
    in symphony's path (`otel.go:236–262`).
  - Span and metric instrument names: any rename — including
    `OllamaLatency → ModelLatency` — is on-wire visible.
  - The OTLP gRPC exporter wire format itself
    (`otlptracegrpc.New`, `otlpmetricgrpc.New`,
    `otlploggrpc.New`).
- **Golden-file location.** Named in ADR-0001 — pull the current path
  at the time of the ADR-0009 PR rather than baking it into this
  document.
- **Required workflow.** Any advisor PR that migrates onto
  `agent-otel` MUST:
  1. Regenerate the replay corpus against the new init path.
  2. Get advisor-maintainer sign-off on the diff against the previous
     corpus (it will not be empty; the diff is expected to be exactly
     `service.version` literal → `semconv.ServiceVersion` helper plus
     the `deployment.environment` addition).
  3. Update ADR-0001's clauses 16–23 to point to ADR-0009 as the
     superseding decision.
  4. Update the replay-gate's golden file in the same commit.
- Not optional, not deferrable, not workable around. If a maintainer
  raises a concern about the diff, the migration PR holds.

---

## 10. Versioning and release plan

- **`v0.1.0`.** First tagged release. Contains the lifted code from
  symphony with the §3 pre-extraction items done. Advisor migrates
  against this version with the regenerated MCP replay corpus.
  Symphony migrates in the same release window or the next.
- **`v0.2.0`.** Adds the `Usage*` histograms wired through to
  symphony's Phase 4 token-accounting work
  (`~/docs/eino/04-integration-plan.html`, Phase 4). Adds any new
  span attribute constants needed by the Anthropic / OpenAI / Google
  provider plugins.
- **`v0.3.0` – `v0.x`.** Iterate on the cardinality-budget runtime
  enforcement question (§14) and on log-signal stability as
  `otel/log` itself approaches GA upstream.
- **`v1.0.0`.** Tag only after **≥3 months** of stable consumption in
  both advisor and symphony. The 3-month window is longer than the
  other modules in the four-repo proposal because OTel instrument
  names ossify dashboards across every operator that touches the
  data — renames after `v1.0.0` are effectively impossible without a
  major version bump.
- **License.** Undecided. Same Phase 0 TODO as the other shared
  repos — see open questions in §14.

---

## 11. Test parity

Lift the symphony test suite as-is (it is the more comprehensive one)
and port the advisor no-op timing test on top:

- `endpoint_explicit_test.go` — `EndpointExplicit` derivation.
- `otel_test.go` — bootstrap / init paths.
- `otel_integration_test.go` — exporter wiring against a collector stub.
- `cleanup_test.go` — coordinated shutdown ordering.
- `cardinality_test.go` — allowlist / prohibition tests.
- `instruments_test.go` — instrument-bundle construction.
- `slog_test.go` — the trace-context handler + tee handler.
- `spans_test.go` — canonical span-name constants.
- `config_test.go` — env-var parsing, validation, normalisation,
  header parsing, the full `ConfigValidationError` surface.

Add from `~/git/advisor/internal/telemetry/telemetry_test.go`: the
no-op fallback timing test — verifies that on OTLP dial failure
within `dialTimeout`, `Init` returns no-op providers in less than
`dialTimeout` rather than waiting for the full per-RPC timeout.

Migration-window tests for v0.1.0:

- Round-trip equivalence: no-op `Init` produces the same
  `*Providers` shape as a real `Init` + immediate `Shutdown`.
- Service-version attribute is set via
  `semconv.ServiceVersion(opts.Version)` — catches regressions to
  the old literal shape that would re-pin a future caller to legacy
  bytes.

---

## 12. Risk register

1. **Byte-equal MCP replay-gate failure on advisor migration.** The #1
   risk by a wide margin. Mitigation: §3b lands and is signed off
   before any consumer-migration PR opens; the regenerated corpus is
   audited diff-by-diff by an advisor maintainer.
2. **semconv version churn upstream.** `v1.40.0` is current;
   `v1.41+` will likely follow. Pick `v1.40.0` for v0.1.0 and do not
   chase head until the next tagged release.
3. **Cardinality-budget regression.** Test-only enforcement means a
   forgotten `RegisterAllowedLabels` call only blows up in
   production. Mitigation: ship a recommended smoke test in
   `agent-otel/testdata/` that downstream test suites import directly.
4. **Dashboard breakage when `OllamaLatency → ModelLatency`.**
   Symphony's existing dashboards consume the old name. Either ship a
   backward-compat alias in v0.1.0 (§14) or coordinate a dashboard
   rewrite as part of symphony's migration PR.
5. **Log-signal stability.** `otel/log` is still pre-1.0 in some SDKs.
   Mitigation: gate behind `opts.EnableLogs` and pin the upstream
   version explicitly.
6. **Module-path collision.** Proposed `github.com/mattsp1290/agent-otel`.
   Quick `go list` search before tagging.
7. **Global-provider race in tests.** Symphony already has
   `SkipGlobalInstall` (`config.go:230–238`); the extraction
   preserves it as `Options.SkipGlobalInstall`. Risk: advisor's
   tests do not use it yet and will need updating.

---

## 13. First-PR breakdown

Five sequential PRs, each scoped tight enough for one reviewer to load:

1. **PR1 — symphony semconv bump.** Move
   `~/git/local-symphony/internal/obs/otel.go:24` from
   `semconv/v1.26.0` to `semconv/v1.40.0`. Run the full test suite
   plus a manual dashboard sweep. No `agent-otel` content. Ships
   independently and stays shipped if the rest of the extraction
   slips.
2. **PR2 — advisor ADR-0009 + corpus regeneration.** Write the new
   ADR that supersedes ADR-0001 clauses 16–23 and permits
   `semconv.ServiceVersion`. Regenerate the byte-equal MCP replay
   corpus. Update the golden file. Get advisor-maintainer sign-off.
   No code change to `telemetry.go` yet — that lands in PR4.
3. **PR3 — agent-otel skeleton.** Initial commit of the new repo
   containing `bootstrap.go`, `config.go`, `cardinality.go`,
   `instruments.go`, `spans.go`, `slogbridge.go`, `cleanup.go`, and
   the lifted test suite from §11. Tag `v0.0.1` as a pre-release for
   integration testing only. Use the unified env-var precedence
   rule from §8.
4. **PR4 — advisor migration to agent-otel v0.1.0.** Tag agent-otel
   `v0.1.0`. Advisor's `internal/telemetry/` shrinks to a thin
   wrapper plus the product-specific advisor-call histogram.
   Re-run the regenerated MCP replay corpus from PR2 against the
   migrated init path; gate passes.
5. **PR5 — symphony migration + Usage histograms.** Symphony's
   `internal/obs/` shrinks; the generic instruments migrate to
   `agent-otel`. Phase 4 token-accounting metrics (`UsageInputTokens`,
   `UsageOutputTokens`) land in agent-otel `v0.2.0` and symphony
   wires through. Dashboards updated (or alias shipped — see §14).

---

## 14. Open questions / TODOs for human review

- **License choice.** Same Phase 0 TODO as the other shared repos.
  Recommend Apache 2.0 to match upstream OTel contrib; defer until
  the umbrella decision lands.
- **Backward-compat alias for `OllamaLatency → ModelLatency`.** Ship a
  deprecated `OllamaLatency` alias in v0.1.0 and drop in v0.2.0?
  Avoids breaking symphony's existing dashboards. Decision needed
  before PR3.
- **Grafana dashboard JSON in-repo.** Publish a starter dashboard so
  the metric conventions are concrete and the cardinality budget is
  visible to operators. Likely defer to v0.2.0.
- **Log-signal support in v0.1.0?** Currently included in §4 layout
  but gated via `opts.EnableLogs`. Question: ship logs as a
  sub-package `agent-otel/logs` so the core boot path does not pull
  `otelslog` into every consumer? Defer until upstream `otel/log` GA.
- **Cardinality budget: hard-coded or per-Options?** Symphony surfaces
  an allowlist, not a numeric cap. Add a numeric
  `Options.CardinalityBudget` on top of the allowlist? Defer to v0.2.0
  once Phase 4 emits enough labels to measure.
- **Env-var precedence flip rollout.** The §3c flip from symphony's
  "general wins" to the OTel-spec "per-signal wins" rule is a real
  behaviour change. Emit a deprecation warning at boot when both
  general and per-signal are set in v0.0.1 (PR3); flip behaviour in
  v0.1.0 (PR4).

---

## Summary

This plan lives at
`/Users/punk1290/git/agent-otel/docs/prompts/extract-agent-otel.md`.
**Next step:** human review of §3 (the three pre-extraction items)
and §14 (open questions); none of §4 and below begins until §3a–c
have explicit owners and a target landing window.
