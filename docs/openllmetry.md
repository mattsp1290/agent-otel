# OpenLLMetry Compatibility

`agent-otel` emits OpenTelemetry GenAI semantic-convention attributes by
default. OpenLLMetry compatibility is additive and default-off:

```go
WithOpenLLMetryCompat()
```

The option adds legacy `llm.*` and Traceloop workflow attributes where
`agent-otel` has enough information to do so without weakening the native
`gen_ai.*` shape. It must not remove, rename, or suppress OTel-native
attributes.

Verified on 2026-05-25 against current OpenLLMetry/Traceloop docs and source:

- OpenLLMetry is OpenTelemetry-based and can export to existing observability
  stacks.
- Current JS semantic-convention constants retain several `llm.*` attributes
  for backwards compatibility, including `llm.request.type`,
  `llm.usage.total_tokens`, `llm.request.functions`, and sampling fields such
  as `llm.top_k`.
- Traceloop workflow/span keys remain `traceloop.span.kind`,
  `traceloop.workflow.name`, `traceloop.entity.name`,
  `traceloop.entity.path`, `traceloop.entity.version`,
  `traceloop.entity.input`, `traceloop.entity.output`, and
  `traceloop.association.properties`.

## Default Behavior

Default mode emits only OTel-native attributes and events:

- `gen_ai.system`
- `gen_ai.provider.name`
- `gen_ai.request.model`
- `gen_ai.response.model` when known
- `gen_ai.operation.name`
- `gen_ai.usage.input_tokens`
- `gen_ai.usage.output_tokens`
- content-bearing GenAI event attributes only when payload capture is enabled
  through the redaction contract in `docs/redaction.md`

No `llm.*` or `traceloop.*` attributes are emitted unless
`WithOpenLLMetryCompat()` is set.

## Compatibility Mapping

| OTel-native source | Compat attribute | Behavior |
| --- | --- | --- |
| `gen_ai.operation.name=chat` | `llm.request.type=chat` | Emit in compat mode. |
| `gen_ai.operation.name=text_completion` | `llm.request.type=completion` | Emit in compat mode using OpenLLMetry's legacy value. |
| `gen_ai.operation.name` other value | `llm.request.type=unknown` | Emit `unknown` rather than copying high-cardinality operation names. |
| `gen_ai.usage.input_tokens` + `gen_ai.usage.output_tokens` | `llm.usage.total_tokens` | Emit sum only when usage is available. |
| `gen_ai.tool.definitions` | `llm.request.functions` | Emit only when payload/tool-definition capture is enabled and redacted. |
| `gen_ai.request.model` | no default legacy duplicate | Keep native key. Do not emit older ad hoc model aliases unless implementation-time source verification requires one. |
| `gen_ai.provider.name` / `gen_ai.system` | no default legacy duplicate | Keep native provider keys. |
| `gen_ai.agent.name` or caller-supplied workflow name | `traceloop.entity.name` | Emit only when caller provides an entity/workflow name. |
| caller-supplied workflow name | `traceloop.workflow.name` | Emit only for workflow-level spans. |
| span role: workflow/task/agent/tool/session | `traceloop.span.kind` | Emit bounded enum value in compat mode. |
| caller-supplied entity path | `traceloop.entity.path` | Emit only when caller provides a bounded path. |
| caller-supplied entity version | `traceloop.entity.version` | Emit only when caller provides a stable version. |
| payload input/output | `traceloop.entity.input` / `traceloop.entity.output` | Do not emit from raw prompts by default; if supported later, values must pass the same payload-capture and redaction gate. |
| caller-supplied association map | `traceloop.association.properties` | Emit only from explicit caller-provided low-cardinality metadata. |

Do not emit compatibility attributes for values `agent-otel` cannot derive
safely. Compatibility mode is not a license to copy every span attribute into a
legacy namespace.

## Overlap Rules

- Native `gen_ai.*` attributes are authoritative.
- Compatibility attributes are derived from native values after validation.
- If a native value and a caller-supplied compat value disagree, prefer the
  native value and record a test-visible validation error in strict mode.
- Compatibility attributes must pass the same cardinality checks as native
  metric labels when used on metrics.
- Prompt, completion, system-instruction, and tool-definition payloads must pass
  through `PromptRedactor` before any legacy content field is emitted.
- Compatibility mode must not enable payload capture.

## Public Options

```go
type OpenLLMetryCompatOptions struct {
	WorkflowName string
	EntityName   string
	EntityPath   string
	EntityVersion string
	SpanKind     TraceloopSpanKind
	AssociationProperties map[string]string
}

func WithOpenLLMetryCompat(opts ...OpenLLMetryCompatOptions) Option
```

`SpanKind` is a bounded enum:

```go
const (
	TraceloopWorkflow TraceloopSpanKind = "workflow"
	TraceloopTask     TraceloopSpanKind = "task"
	TraceloopAgent    TraceloopSpanKind = "agent"
	TraceloopTool     TraceloopSpanKind = "tool"
	TraceloopSession  TraceloopSpanKind = "session"
	TraceloopUnknown  TraceloopSpanKind = "unknown"
)
```

Unset workflow/entity fields should not produce empty attributes.

## Tests

Implementation tests must assert both modes exactly.

Default native-only mode:

- A model-call span includes `gen_ai.request.model`,
  `gen_ai.provider.name`, `gen_ai.operation.name`, and usage attributes when
  usage is available.
- The same span does not include any `llm.*` attributes.
- The same span does not include any `traceloop.*` attributes.
- Payload capture remains disabled.

Compatibility mode:

- Native `gen_ai.*` attributes are still present.
- `gen_ai.operation.name=chat` produces `llm.request.type=chat`.
- `gen_ai.operation.name=text_completion` produces
  `llm.request.type=completion`.
- Available usage produces `llm.usage.total_tokens=input+output`.
- Unavailable usage does not produce `llm.usage.total_tokens`.
- Caller-provided workflow/entity options produce the corresponding
  `traceloop.*` attributes.
- Empty workflow/entity options produce no empty `traceloop.*` attributes.
- Tool definitions produce `llm.request.functions` only when payload capture is
  enabled and the redacted value is used.
- Compatibility mode alone does not emit prompt/completion payload content.

Regression tests should include one fixture that captures all span attributes
and fails if native keys disappear when compatibility mode is enabled.

## References

- https://www.traceloop.com/docs/openllmetry/introduction
- https://raw.githubusercontent.com/traceloop/openllmetry-js/main/packages/ai-semantic-conventions/src/SemanticAttributes.ts
