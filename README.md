# agent-otel

`agent-otel` is a small Go library for OpenTelemetry bootstrap and
agent-oriented telemetry helpers. It builds trace, metric, and log providers;
emits OpenTelemetry GenAI spans and metrics; and keeps vendor presets,
cardinality filtering, and prompt payload capture explicit.

Current consumers:

- [`advisor`](https://github.com/mattsp1290/advisor)
- [`local-symphony`](https://github.com/mattsp1290/local-symphony)

## Install

```bash
go get github.com/mattsp1290/agent-otel
```

## Basic Bootstrap

```go
providers, shutdown, err := agentotel.Init(ctx, agentotel.Options{
	Enabled:           true,
	ServiceName:       "my-agent",
	ServiceVersion:    "1.2.3",
	Environment:       "production",
	SkipGlobalInstall: false,
})
if err != nil {
	return err
}
defer shutdown.Shutdown(context.Background())
```

`Init` returns non-nil no-op providers and instruments when `Enabled=false`.
When exporters are enabled, `agent-otel` follows OpenTelemetry OTLP env-var
precedence:

```text
explicit Options field
> per-signal OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_*
> general OTEL_EXPORTER_OTLP_*
> default
```

See [docs/env-vars.md](docs/env-vars.md) for endpoint, protocol, header,
timeout, TLS, Datadog, and lotel dev-sink examples.

## GenAI Spans

Use span helpers to emit OTel GenAI attributes without scattering raw string
keys through consumers:

```go
ctx, span, err := agentotel.StartModelCall(ctx, providers.Tracer, agentotel.ModelCall{
	OperationName: agentotel.GenAIOperationChat,
	ProviderName:  "openai",
	RequestModel:  "gpt-4o",
	Usage: agentotel.Usage{
		InputTokens:  120,
		OutputTokens: 80,
		Available:    true,
	},
})
if err != nil {
	return err
}
defer span.End()
```

Available usage emits `gen_ai.usage.input_tokens` and
`gen_ai.usage.output_tokens`. Unavailable usage emits
`agent_otel.usage.available=false` and omits token counts. See
[docs/genai-mapping.md](docs/genai-mapping.md) and
[docs/usage-boundary.md](docs/usage-boundary.md).

## Metrics

`Providers.Instruments` exposes cardinality-checked recorders for the built-in
model-call metrics:

- `gen_ai.client.operation.duration`, unit `s`
- `gen_ai.client.token.usage`, unit `{token}`
- `agent_otel.provider.errors`, unit `{error}`
- `agent_otel.fallback.engaged`, unit `{event}`

```go
labels := agentotel.ModelMetricLabels{
	OperationName: agentotel.GenAIOperationChat,
	ProviderName:  "openai",
	RequestModel:  "gpt-4o",
}
_ = providers.Instruments.RecordModelLatency(ctx, 0.42, labels)
_ = providers.Instruments.RecordUsage(ctx, agentotel.Usage{
	InputTokens:  120,
	OutputTokens: 80,
	Available:    true,
}, labels)
```

Metric labels are filtered against an allowlist before they reach OTel
instruments. See [docs/cardinality-runtime.md](docs/cardinality-runtime.md).

## Presets And Compatibility

`WithDatadogPreset()` adds Datadog LLM Observability-friendly trace defaults
without changing the vendor-neutral GenAI span shape. It sets OTLP/HTTP trace
defaults and adds Datadog trace headers from explicit options or `DD_API_KEY`.
See [docs/datadog.md](docs/datadog.md).

`WithOpenLLMetryCompat()` adds legacy `llm.*` and `traceloop.*` attributes in
addition to native `gen_ai.*` attributes. It is default-off and does not enable
payload capture. See [docs/openllmetry.md](docs/openllmetry.md).

`WithDevSink(lotelEndpoint)` duplicates telemetry to a local
[`lotel`](https://github.com/mattsp1290/lotel) OTLP endpoint while preserving
the primary exporter path. Missing local lotel fails open. Filed lotel requests
are tracked in [docs/lotel-requests.md](docs/lotel-requests.md).

## Payload Capture

Prompt, completion, system-instruction, and tool-definition payloads are not
captured by default. `WithPayloadCapture(redactor, ...)` opt-in is required,
and the redactor runs before payloads are attached to span events. See
[docs/redaction.md](docs/redaction.md).

## Migration References

Consumer migration decisions and old-to-new telemetry mappings are documented
in [docs/telemetry-migration-map.md](docs/telemetry-migration-map.md). The
bootstrap design history lives in [docs/bootstrap-design.md](docs/bootstrap-design.md).

## Development

```bash
go test ./...
```
