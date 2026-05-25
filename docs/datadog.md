# Datadog LLM Observability

`agent-otel` should emit vendor-neutral OpenTelemetry GenAI spans by default.
Datadog LLM Observability can ingest those spans when they follow the OTel
GenAI 1.37+ semantic conventions. A Datadog preset may configure exporter
defaults and headers, but Datadog-specific behavior must remain optional.

Verified on 2026-05-25 against Datadog's current OpenTelemetry LLM
Observability documentation:

- Datadog supports OpenTelemetry traces that follow GenAI semantic conventions
  1.37+.
- Direct LLM Observability ingestion uses OTLP traces over HTTP/protobuf.
- Trace headers include `dd-api-key=<api key>` and `dd-otlp-source=llmobs`.
- Frameworks that previously emitted pre-1.37 GenAI conventions may need
  `OTEL_SEMCONV_STABILITY_OPT_IN=gen_ai_latest_experimental`.

## Required Attribute Mapping

| OTel attribute | Datadog LLM Observability field | agent-otel requirement |
| --- | --- | --- |
| `gen_ai.operation.name` | `meta.span.kind` via Datadog span-kind resolution | Always emit on model, embedding, tool, and agent spans. |
| `gen_ai.provider.name` | `meta.model_provider` | Emit provider when known. Datadog falls back to `gen_ai.system`, then `custom`. |
| `gen_ai.request.model` | `meta.model_name` fallback | Emit requested model on every model call. |
| `gen_ai.response.model` | `meta.model_name` preferred | Emit when the provider returns the actual model. |
| `gen_ai.usage.input_tokens` | `metrics.input_tokens` | Emit only when normalized usage is available. |
| `gen_ai.usage.output_tokens` | `metrics.output_tokens` | Emit only when normalized usage is available. |

Datadog resolves `gen_ai.operation.name` to span kind as follows:

| `gen_ai.operation.name` | Datadog span kind |
| --- | --- |
| `generate_content`, `chat`, `text_completion`, `completion` | `llm` |
| `embeddings`, `embedding` | `embedding` |
| `execute_tool` | `tool` |
| `invoke_agent`, `create_agent` | `agent` |
| `rerank`, `unknown`, or missing/default | `workflow` |

For `agent-otel`, model calls should use `chat`, `text_completion`, or
`generate_content` consistently with `docs/genai-mapping.md`. Tool and agent
helpers should use `execute_tool` and `invoke_agent` when applicable.

## Request, Response, and Content Fields

Datadog maps all `gen_ai.request.*` parameters to metadata with the prefix
stripped. The preset should not require these, but helper APIs may set bounded
request attributes such as:

- `gen_ai.request.max_tokens`
- `gen_ai.request.temperature`
- `gen_ai.request.top_p`
- `gen_ai.request.stop_sequences`

Datadog maps response fields:

- `gen_ai.response.model` to model name.
- `gen_ai.response.finish_reasons` to metadata finish reasons.

Datadog extracts input/output messages from direct attributes first, then from
span events named:

```text
gen_ai.client.inference.operation.details
```

`agent-otel` should prefer span events for payload content and must apply
`docs/redaction.md` before emitting any content-bearing field:

- `gen_ai.input.messages`
- `gen_ai.output.messages`
- `gen_ai.system_instructions`
- `gen_ai.tool.definitions`

Payload capture stays opt-in. The Datadog preset must not enable prompt or
completion capture by itself.

## WithDatadogPreset

`WithDatadogPreset()` should set Datadog-friendly defaults without making the
core library vendor-specific:

```go
func WithDatadogPreset(opts ...DatadogPresetOption) Option
```

Defaults:

- Prefer OTLP/HTTP for traces.
- Set traces protocol to `http/protobuf`.
- Add trace headers:
  - `dd-api-key` from `DD_API_KEY`.
  - `dd-otlp-source=llmobs`.
- If an explicit Datadog OTLP traces endpoint is supplied, use it.
- If no endpoint is supplied, respect standard OTel endpoint environment
  variables. Do not silently override a caller-configured collector.
- Set or recommend `OTEL_SEMCONV_STABILITY_OPT_IN=gen_ai_latest_experimental`
  for callers that need latest experimental GenAI conventions.
- Keep payload capture off.
- Keep `WithOpenLLMetryCompat()` off unless the caller also requests it.

Endpoint behavior:

- For direct Datadog intake, use the site-specific OTLP traces endpoint for the
  organization, for example `https://otlp.datadoghq.com/v1/traces` on US1.
- For Datadog Agent or collector use, respect caller-provided
  `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` or `OTEL_EXPORTER_OTLP_ENDPOINT`.
- The preset should expose an explicit endpoint override rather than guessing
  the Datadog site from partial configuration.

## On-Wire Tests

Implementation tests should use the in-process OTLP receiver and assert exact
serialized attributes.

Required default tests:

- Model-call span includes `gen_ai.operation.name=chat`,
  `gen_ai.provider.name=openai`, and `gen_ai.request.model=gpt-4o`.
- Usage available emits `gen_ai.usage.input_tokens=100` and
  `gen_ai.usage.output_tokens=180`.
- Usage unavailable omits both token attributes and records the local
  availability marker defined in `docs/usage-boundary.md`.
- Response model, when set, emits `gen_ai.response.model`.
- Payload content is absent unless payload capture is explicitly enabled.

Required preset tests:

- `WithDatadogPreset()` selects OTLP/HTTP trace exporter configuration.
- It adds `dd-api-key` from `DD_API_KEY`.
- It adds `dd-otlp-source=llmobs`.
- It does not overwrite an explicit collector endpoint.
- It does not enable payload capture.
- It does not enable OpenLLMetry compatibility.
- It preserves all native `gen_ai.*` attributes.

Required content tests:

- With payload capture disabled, no `gen_ai.input.messages` or
  `gen_ai.output.messages` appears on spans or events.
- With payload capture enabled, content appears only on
  `gen_ai.client.inference.operation.details` after redaction.
- Datadog-compatible event output uses structured values when the Go OTel event
  API/exporter supports them, otherwise a documented JSON string fallback.

## References

- https://docs.datadoghq.com/llm_observability/instrumentation/otel_instrumentation/
- https://www.datadoghq.com/blog/llm-otel-semantic-convention/
- https://docs.datadoghq.com/opentelemetry/setup/otlp_ingest/metrics/
