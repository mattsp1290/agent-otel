# Telemetry Migration Map

This map records how advisor and local-symphony telemetry should cut over to
`agent-otel` helpers and OTel GenAI output. It is based on read-only inspection
of `/home/infra-admin/git/advisor` and
`/home/infra-admin/git/local-symphony` on 2026-05-25.

The migration intentionally replaces project-local model-call names with
GenAI-compatible spans and attributes. Product-specific fields remain only when
they are useful and bounded.

## Advisor

Current advisor files:

- `internal/advisor/core/advise.go`
- `internal/advisor/core/span_parity_test.go`
- `internal/advisor/core/advise_test.go`
- `internal/telemetry/telemetry.go`
- `internal/telemetry/cli_init.go`

| Current surface | Target API | Retained attrs | Dropped attrs | Required tests |
| --- | --- | --- | --- | --- |
| Span `advisor.handle` | `agentotel.StartModelCall(ctx, ModelCall{Operation:"chat", Provider:req.Provider, Model:req.Model})` or equivalent span helper | `error.type`; bounded `advisor.entrypoint` as product attr or `agent_otel.entrypoint`; request provider/model values via GenAI keys | Span name `advisor.handle` | Span test asserts `gen_ai.operation.name=chat`, `gen_ai.provider.name`, `gen_ai.request.model`, retained entrypoint attr, and no `advisor.handle` span name. |
| `advisor.provider` | Model-call labels and span attrs | Value retained as `gen_ai.provider.name`; optionally `gen_ai.system` when mapping doc requires provider system identifier | `advisor.provider` | Success and provider-error tests assert provider appears only under GenAI key plus any documented compatibility key. |
| `advisor.model` | Model-call labels and span attrs | Value retained as `gen_ai.request.model`; `gen_ai.response.model` only when provider returns actual model | `advisor.model` | Success test asserts requested model; response-model test added when provider response model is plumbed. |
| `advisor.input_tokens` / `advisor.output_tokens` | `Instruments.RecordUsage(ctx, agentotel.Usage, labels)` and `SetUsageSpanAttributes` | Values retained as `gen_ai.usage.input_tokens` and `gen_ai.usage.output_tokens` when `Usage.Available=true` | `advisor.input_tokens`, `advisor.output_tokens` | Tests cover available usage emits token attrs/metrics; unavailable usage omits token attrs/metrics. |
| `advisor.usage_available` | Usage boundary availability marker | Boolean retained as `agent_otel.usage.available` or finalized local key from `docs/usage-boundary.md` | `advisor.usage_available` | Tests distinguish unavailable usage from reported zero usage. |
| `advisor.latency_ms` span attr | `RecordModelLatency(ctx, seconds, labels)` and span duration | Latency retained as metric value in seconds; no required span attr because span duration already carries wall time | `advisor.latency_ms` | Histogram/OTLP test asserts model latency metric exists with GenAI labels; span duration remains non-zero in fake clock test where feasible. |
| Histogram `advisor.call.duration` unit `ms` labels `provider`, `model`, `entrypoint` | `RecordModelLatency` wrapper | Provider/model retained under GenAI metric labels; entrypoint retained only if cardinality design allows product label | Metric name `advisor.call.duration`; raw labels `provider`, `model` | Metric test asserts new metric name/unit from `docs/genai-mapping.md`, no old metric, allowed labels only. |
| `advisor.entrypoint` values `mcp`, `cli`, `unknown` | Product attr on model-call span and optional metric label | Values retained because they are bounded and existing CLI/MCP parity tests depend on symmetry | none if renamed to `agent_otel.entrypoint`; old key may be dropped | Parity test updated so MCP/CLI spans still differ only by entrypoint value. |
| `error.type` on validation/provider/panic paths | OTel error attr plus span status/recorded exception | Retain exact `ErrType` string values | none | Existing `TestCore_Advise_SpanErrorType_IsWireFormat` is updated for new span helper and must keep exact values. |
| Validation error paths with only `advisor.entrypoint` + `error.type` | Agent/model span helper or validation span helper | Keep entrypoint and error.type; do not add provider/model when validation failed before those values are valid | advisor-prefixed keys | Validation tests assert missing-provider/missing-model span shape does not invent provider/model GenAI attrs. |
| Provider init/factory error path with provider/model attrs | Model-call span helper | Keep provider/model under GenAI keys and `error.type=provider.init_error` or auth error | advisor-prefixed provider/model | Provider init tests assert GenAI attrs and error status. |
| Panic path skips histogram | Model-call helper with panic-safe defer | Preserve no-latency-metric behavior if operation panics before a valid provider call completes | old histogram skip assertion wording | Panic test asserts no model-latency metric is recorded and span has error status. |
| `InitServe`, `InitCLI`, `InitDisabled`, `Shutdown.ForceFlush`, `Shutdown.Shutdown` | `agentotel.Init(ctx, Options)` plus advisor thin wrappers | CLI timeout behavior, serve defaults, no-op disabled mode, explicit ForceFlush then Shutdown contract | Duplicated advisor telemetry bootstrap implementation | Telemetry tests assert CLI bounded flush/shutdown, disabled no-op providers, no-op fallback on exporter init failure, resource attrs. |

Advisor migration notes:

- Keep advisor's `Provider.Advise` usage extraction in advisor; convert to
  `agentotel.Usage` only at telemetry emission.
- Preserve `ErrType` literals; dashboards and exit-code behavior depend on
  those values even if span names change.
- Entry point is product-specific but bounded. Retain it as a product attr
  unless the final cardinality table rejects it from metrics.

## local-symphony

Current symphony files:

- `internal/obs/spans.go`
- `internal/obs/instruments.go`
- `internal/obs/cardinality.go`
- `internal/worker/agent/graph.go`
- `internal/worker/agent/fallback_signal.go`
- `internal/worker/agent/validator.go`
- `internal/worker/agent/fallback_signal_test.go`

| Current surface | Target API | Retained attrs | Dropped attrs | Required tests |
| --- | --- | --- | --- | --- |
| Span `symphony.agent.turn` attrs `model.name`, `session.id`, `thread.id`, `turn.id` | `agentotel.StartAgentSpan` / `StartModelCall` around the Eino graph turn | Model retained as `gen_ai.request.model`; session/thread/turn retained as span-only product attrs if needed for correlation | Span name `symphony.agent.turn`; `model.name` | Span test asserts GenAI model attrs plus span-only correlation attrs; cardinality test confirms correlation attrs are never metric labels. |
| Token counters `symphony.tokens.aggregate` and `symphony.tokens.per_turn` labels `model.name`, `direction` | `RecordUsage(ctx, Usage, ModelLabels)` | Prompt/completion counts retained as `gen_ai.usage.input_tokens` and `gen_ai.usage.output_tokens`; model retained as `gen_ai.request.model` | Metric names `symphony.tokens.*`; label `direction`; label `model.name` | Graph/runtime tests assert Eino usage maps to available `Usage`; metric tests assert input/output token series and no old symphony token metrics. |
| Graph `Output.Tokens` extracted from `schema.Message.ResponseMeta.Usage` | Consumer-side conversion to `agentotel.Usage` | Non-negative counts retained; explicit availability bit added in symphony migration | Implicit missing-usage-as-zero behavior | Tests assert missing `ResponseMeta.Usage` remains distinguishable from reported zero usage. |
| Span `symphony.tool.call` attrs `tool.name`, `tool.outcome`, event for `tool.input` | `agentotel.StartToolSpan` or equivalent | `gen_ai.tool.name`; tool outcome retained as product attr; redacted tool input via payload event only | Span name `symphony.tool.call`; raw `tool.input/*` attrs | Tool tests assert `execute_tool` operation, tool name, outcome, and redaction of tool input. |
| Metrics `symphony.tool_calls` labels `tool.name`, `outcome` | `AddToolCall(ctx, ToolLabels)` if implemented in agent-otel, otherwise symphony-owned project metric registered with allowlist | `tool.name` value retained as `gen_ai.tool.name` on spans and bounded metric label if allowed; outcome retained | Old metric name if replaced by shared helper | Metric tests assert allowed labels and outcome enum literals `succeeded`/`failed` or validator literals as applicable. |
| Metric `symphony.tool_calls.malformed` labels `model.name`, `tool.name` | `AddToolCallMalformed` or symphony-owned registered metric | Model retained as `gen_ai.request.model`; tool name retained; validator outcomes retained as bounded product attrs | `model.name` label | Validator tests keep `OutcomeValid`, `OutcomeUnsupported`, `OutcomeMalformed`, and `ToolNameUnnamed` literal pins. |
| Fallback event `model_fallback_engaged` plus metric `symphony.model.fallback_engaged_total` labels `model.from`, `model.to` | `AddFallbackEngaged(ctx, FallbackLabels)` plus existing dispatcher event | From/to retained as `agent_otel.provider.from` / `agent_otel.provider.to` or finalized fallback labels; dispatcher event retained | Metric name `symphony.model.fallback_engaged_total`; labels `model.from`, `model.to` | `fallback_signal_test` asserts event still fires, counter increments, new attrs carry from/to values, nil/cancel behavior unchanged. |
| Span `symphony.ollama.request` attrs `model.name`, `request_kind`, `error.kind` | Generic model-call span for chat/generate; project span or registered metric for non-LLM Ollama endpoints | Chat model requests retained as `gen_ai.request.model`, `gen_ai.provider.name=ollama`, `gen_ai.operation.name`; `error.kind` maps to `error.type` when provider error | Span name `symphony.ollama.request`; `request_kind` for model calls | Tests assert chat requests become GenAI model-call spans; embeddings/tags either map to documented operation names or remain symphony project spans. |
| Metrics `symphony.ollama.latency` label `request_kind` and `symphony.ollama.errors` label `error.kind` | `RecordModelLatency` and `AddProviderError` for chat/generate paths; custom registered metrics for non-model Ollama endpoints | Latency retained in seconds; errors retained as `error.type`/bounded error kind | Old metric names for model-call paths; `request_kind` on generic model metrics | Metric tests assert model-call latency/error metrics use GenAI labels; non-model endpoints have explicit registered project metrics if still needed. |
| Span `symphony.poll.tick`, `symphony.dispatch`, `symphony.tracker.query`, `symphony.persistence.write` | Stay symphony-owned or custom registered metrics/spans outside core GenAI helpers | Existing bounded attrs retained (`tracker.kind`, `outcome`, `tracker.op`, `write.kind`, `op`) | Nothing required by agent-otel GenAI migration | Existing obs tests remain in symphony; agent-otel migration should not force these into GenAI helpers. |
| `obs.Instruments` exported raw OTel handles | `agentotel.Instruments` wrapper recorders plus optional custom metric registration | Instrument construction behavior retained; duplicate-instrument avoidance retained | Direct raw metric handles for built-in model/tool/fallback metrics | Tests assert callers use wrapper APIs and cardinality filtering catches prohibited labels. |

Symphony migration notes:

- Keep dispatcher events and persistence schema stable unless a separate
  symphony migration explicitly changes product events.
- Add an explicit usage availability bit near `agent.Output` before mapping to
  `agentotel.Usage`.
- Fallback logs currently include high-cardinality issue/session/run IDs; those
  remain logs, not metric labels.
- Non-GenAI operational metrics such as poll, queue, retries, and persistence
  can stay in symphony or use custom registered metrics. They are not blockers
  for the shared GenAI helper implementation.

## Shared Verification

Both migrations should include:

- OTLP receiver tests asserting raw serialized span and metric attribute names.
- Negative tests proving old advisor/symphony model-call metric names are gone
  from the migrated paths.
- Cardinality tests proving `issue.id`, `session.id`, `thread.id`, `turn.id`,
  `run_attempt.id`, and `tool.input/*` do not appear as metric labels.
- Payload tests proving prompt/tool content goes through the redaction contract
  before any GenAI content event is emitted.
- Datadog mapping tests for
  `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`,
  `gen_ai.usage.input_tokens`, and `gen_ai.usage.output_tokens`.
