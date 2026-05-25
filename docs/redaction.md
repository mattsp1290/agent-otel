# Redaction and Payload Capture

`agent-otel` treats prompt, completion, tool, and system-instruction
payloads as sensitive by default. The library emits GenAI spans and
metrics without payload bodies unless the caller explicitly opts in to content
capture.

This design follows the OpenTelemetry GenAI convention shape current on
2026-05-25: GenAI conventions are still Development status, and content-bearing
attributes such as `gen_ai.input.messages`, `gen_ai.output.messages`,
`gen_ai.system_instructions`, and `gen_ai.tool.definitions` are Opt-In. The
content event currently named `gen_ai.client.inference.operation.details` is
also opt-in.

## Defaults

- Payload capture is off by default.
- Disabling payload capture must still allow non-sensitive span attributes,
  metrics, logs, and error status to be emitted.
- Enabling payload capture requires an explicit option such as
  `WithPayloadCapture(...)`; setting only the exporter, Datadog preset, dev
  sink, or OpenLLMetry compatibility option must not enable payload capture.
- Payload capture applies to prompt input, model output, system instructions,
  and tool definitions. Future content-bearing GenAI fields inherit the same
  default-deny behavior.
- Unredacted payloads are never logged. This includes normal logs, debug logs,
  redactor error logs, exporter fallback logs, and test failure helper output.

## Public Hook

The redaction hook receives structured content before any payload is attached
to a span event:

```go
type PayloadKind string

const (
	PayloadPrompt             PayloadKind = "prompt"
	PayloadCompletion         PayloadKind = "completion"
	PayloadSystemInstructions PayloadKind = "system_instructions"
	PayloadToolDefinitions    PayloadKind = "tool_definitions"
)

type PromptPayload struct {
	Kind       PayloadKind
	Provider   string
	Model      string
	Operation  string
	Attributes map[string]any
	Value      any
}

type PromptRedactor interface {
	RedactPrompt(ctx context.Context, payload PromptPayload) (PromptPayload, error)
}
```

`Value` should preserve the structured OTel GenAI schema where the caller can
provide it. If the caller only has a JSON string, the implementation may carry
that string, but tests should prefer structured values for input and output
messages.

`DropAllPayloadsRedactor` drops payload values. `WithPayloadCapture(nil)` uses
that redactor, so payload capture still requires explicit opt-in:

```go
WithPayloadCapture(redactor PromptRedactor)
```

`AllowUnredactedPayloadsRedactor` exists for local debugging and is never
selected by any preset.

## Ordering

The emission path must be:

1. Build the non-sensitive span and metric attributes.
2. If payload capture is disabled, return without constructing content events.
3. Build the content payload value in memory.
4. Call `PromptRedactor.RedactPrompt`.
5. Validate the returned payload against size and type limits.
6. Attach only the redacted payload to the span event.
7. Discard the unredacted local value.

The redactor runs before span events are attached. No API should expose a path
that accepts already-attached events containing raw prompt or completion data.

## Event Shape

Use the OTel GenAI content event when the implementation-time
`docs/genai-mapping.md` confirms it is still current:

```text
gen_ai.client.inference.operation.details
```

The event should contain the redacted structured fields that are applicable to
the operation:

- `gen_ai.input.messages`
- `gen_ai.output.messages`
- `gen_ai.system_instructions`
- `gen_ai.tool.definitions`
- non-sensitive correlation fields already allowed on the span, such as
  `gen_ai.operation.name`, `gen_ai.provider.name`, and
  `gen_ai.request.model`

Do not put raw content on ordinary log records or metric attributes. If a
backend has a legacy OpenLLMetry field for content capture, compatibility mode
must apply the same redacted value and must remain behind the same opt-in gate.

## Error Handling

Redactor errors are privacy failures. The default behavior should be fail-closed:

- Do not attach the content event.
- Record a low-cardinality diagnostic attribute or event that redaction failed,
  without including the payload or redactor error text if it may contain content.
- Return or surface an error only when the caller opted into strict capture.
  Non-strict mode should keep the model operation telemetry path alive while
  dropping the sensitive event.

Recommended options:

```go
type RedactionFailureMode int

const (
	DropPayloadOnRedactionError RedactionFailureMode = iota
	ReturnErrorOnRedactionError
)
```

The fail-closed mode is the default. `ReturnErrorOnRedactionError` is for tests
and callers that want payload capture to be part of the operation contract.

## Size and Type Limits

Payload capture must include limits after redaction:

- Maximum serialized size per event.
- Maximum message count per input or output field.
- Maximum string length per content part.
- Rejection of unsupported value types that cannot be represented by the OTel
  event API or backend exporters.

Truncation markers must not include raw omitted content. A safe marker is a
count or boolean such as `agent_otel.redaction.truncated=true`.

## Privacy Tests

Implementation tests must prove:

- Payload capture is disabled by default.
- Enabling unrelated options or presets does not enable payload capture.
- With capture disabled, no content event is emitted.
- With capture enabled, the redactor runs before the span event is attached.
- The exported event contains the redacted value and not the original value.
- Redactor errors drop the content event in the default mode.
- Strict redactor-error mode returns an error without exporting raw content.
- Logs never contain raw prompt, completion, system-instruction, or tool
  definition values, including redactor error paths.
- OpenLLMetry compatibility mode uses the same redacted payload and does not
  bypass the capture gate.

Use sentinel strings in tests, such as `SECRET_PROMPT_DO_NOT_EXPORT`, and assert
that they are absent from captured spans, events, logs, and test receiver debug
output.
