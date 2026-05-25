# OTel GenAI Mapping

Verified on 2026-05-25 against the official OpenTelemetry GenAI semantic
conventions published as version 1.41.0.

GenAI spans, events, metrics, and most GenAI attributes are Development status.
The implementation must pin literal strings in tests so upstream convention
movement is visible during dependency updates.

## Stability and Opt-In

- GenAI semantic conventions are Development.
- Content-bearing GenAI attributes and the GenAI content event are Opt-In.
- Frameworks emitting older experimental GenAI conventions may need
  `OTEL_SEMCONV_STABILITY_OPT_IN=gen_ai_latest_experimental` to emit latest
  experimental conventions.
- `agent-otel` should emit the latest verified strings by default for its own
  helpers and should document any required environment opt-in for wrapped
  third-party instrumentation.

## Span Names

Use these span names where the helper can identify the operation:

| Operation | Span name | Required attributes |
| --- | --- | --- |
| Chat / text generation | `{gen_ai.operation.name} {gen_ai.request.model}` when model is known | `gen_ai.operation.name`, `gen_ai.request.model`, `gen_ai.provider.name` or `gen_ai.system` |
| Chat / text generation with unknown model | `{gen_ai.operation.name}` | `gen_ai.operation.name`, provider/system when known |
| Tool execution | `execute_tool {gen_ai.tool.name}` | `gen_ai.operation.name=execute_tool`, `gen_ai.tool.name` |
| Agent invocation | `{gen_ai.operation.name} {gen_ai.agent.name}` when agent is known | `gen_ai.operation.name`, `gen_ai.agent.name` |

Recommended operation names for this project:

- `chat` for advisor and symphony chat-model calls.
- `text_completion` only for a non-chat completion API.
- `generate_content` only when matching a provider API with that shape.
- `execute_tool` for tool spans.
- `invoke_agent` for agent-level orchestration spans.
- `embeddings` only if the project later instruments embedding calls.

## Span Attributes

Pin local constants for at least these attributes:

| Constant | Literal | Notes |
| --- | --- | --- |
| `AttrGenAIRequestModel` | `gen_ai.request.model` | Requested model. |
| `AttrGenAIResponseModel` | `gen_ai.response.model` | Actual response model when returned. |
| `AttrGenAIProviderName` | `gen_ai.provider.name` | Provider name for Datadog mapping. |
| `AttrGenAISystem` | `gen_ai.system` | Provider/system identifier when current docs prefer it. |
| `AttrGenAIOperationName` | `gen_ai.operation.name` | Required for Datadog span kind mapping. |
| `AttrGenAIUsageInputTokens` | `gen_ai.usage.input_tokens` | Span/token usage attr, only when usage available. |
| `AttrGenAIUsageOutputTokens` | `gen_ai.usage.output_tokens` | Span/token usage attr, only when usage available. |
| `AttrGenAIUsageTotalTokens` | `gen_ai.usage.total_tokens` | Optional derived attr; not required by Datadog. |
| `AttrGenAIResponseFinishReasons` | `gen_ai.response.finish_reasons` | When provider returns finish reason. |
| `AttrGenAIRequestMaxTokens` | `gen_ai.request.max_tokens` | Optional request param. |
| `AttrGenAIRequestTemperature` | `gen_ai.request.temperature` | Optional request param. |
| `AttrGenAIRequestTopP` | `gen_ai.request.top_p` | Optional request param. |
| `AttrGenAIToolName` | `gen_ai.tool.name` | Tool-call spans. |
| `AttrGenAIAgentName` | `gen_ai.agent.name` | Agent spans. |
| `AttrErrorType` | `error.type` | Existing OTel error convention used by both consumers. |

For provider/system values, implementation should normalize only bounded known
providers (`openai`, `anthropic`, `google_genai`, `ollama`, etc.). Unknown
values may still be emitted as provider names on spans, but metric labels must
pass the cardinality allowlist.

## Events and Payload Fields

The content event is opt-in:

```text
gen_ai.client.inference.operation.details
```

Payload-related attributes:

- `gen_ai.input.messages`
- `gen_ai.output.messages`
- `gen_ai.system_instructions`
- `gen_ai.tool.definitions`

`agent-otel` must emit those only when payload capture is enabled and after
`PromptRedactor` runs. See `docs/redaction.md`.

Tool input/output content should not be placed on metric labels. If emitted,
it must be a span event or event attribute behind the same redaction gate.

## Metrics

Use OTel GenAI metric names where the final implementation-time verification
confirms they are still current.

| Metric | Instrument | Unit | Attributes |
| --- | --- | --- | --- |
| `gen_ai.client.operation.duration` | Histogram | `s` | `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`, `error.type` when failed |
| `gen_ai.client.token.usage` | Histogram | `{token}` | `gen_ai.operation.name`, `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.token.type` |
| Provider error counter | Counter, local name until OTel defines one | `{error}` | `gen_ai.provider.name`, `error.type` |
| Fallback-engaged counter | Counter, local name | `{event}` | bounded fallback labels from `docs/cardinality-runtime.md` |

Token metric decision:

- Use one token histogram named `gen_ai.client.token.usage`.
- Record two data points when usage is available:
  - `gen_ai.token.type=input`
  - `gen_ai.token.type=output`
- Do not record zero token data points when usage is unavailable.
- Span attributes still use `gen_ai.usage.input_tokens` and
  `gen_ai.usage.output_tokens` for Datadog mapping.

If the Go semconv package exposes exact stable helpers at implementation time,
wrappers may delegate to them. Until then, define local string constants and
literal tests.

## Local Constants Decision

Create local constants for every string in this document under a file such as
`genai_attrs.go`. Do not rely on raw string literals at call sites.

Reasons:

- The Go semconv package may lag or omit experimental GenAI constants.
- Tests can pin exact serialized names independent of helper availability.
- Datadog and OpenLLMetry compatibility docs depend on literal names.

Required literal tests:

- `gen_ai.request.model`
- `gen_ai.response.model`
- `gen_ai.provider.name`
- `gen_ai.system`
- `gen_ai.operation.name`
- `gen_ai.usage.input_tokens`
- `gen_ai.usage.output_tokens`
- `gen_ai.response.finish_reasons`
- `gen_ai.tool.name`
- `gen_ai.agent.name`
- `gen_ai.input.messages`
- `gen_ai.output.messages`
- `gen_ai.system_instructions`
- `gen_ai.tool.definitions`
- `gen_ai.client.inference.operation.details`
- `gen_ai.client.operation.duration`
- `gen_ai.client.token.usage`
- `gen_ai.token.type`

## Serialized OTLP Assertions

The in-process OTLP receiver tests must assert the serialized wire shape, not
just helper return values.

Required span assertions:

- Model-call span name is `chat gpt-4o` for operation `chat` and model `gpt-4o`.
- Span has `gen_ai.operation.name=chat`.
- Span has `gen_ai.provider.name=openai`.
- Span has `gen_ai.request.model=gpt-4o`.
- Span has `gen_ai.usage.input_tokens=100` and
  `gen_ai.usage.output_tokens=180` when usage is available.
- Usage-unavailable span has no token attrs and has the local availability
  marker from `docs/usage-boundary.md`.
- Failed span has `error.type` and error status.

Required event assertions:

- With payload capture disabled, no
  `gen_ai.client.inference.operation.details` event appears.
- With payload capture enabled, the event name is exactly
  `gen_ai.client.inference.operation.details`.
- Event payload fields are redacted and use the exact keys listed above.

Required metric assertions:

- Latency metric name is `gen_ai.client.operation.duration`.
- Latency unit is `s`.
- Token metric name is `gen_ai.client.token.usage`.
- Token unit is `{token}`.
- Token data points include `gen_ai.token.type=input` and
  `gen_ai.token.type=output` when usage is available.
- No token data points are emitted when usage is unavailable.
- No metric label includes prohibited high-cardinality keys from
  `docs/cardinality-runtime.md`.

## References

- https://opentelemetry.io/docs/specs/semconv/gen-ai/
- https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-spans/
- https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-agent-spans/
- https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-events/
- https://opentelemetry.io/docs/specs/semconv/gen-ai/gen-ai-metrics/
