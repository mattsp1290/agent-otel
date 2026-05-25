# Bootstrap Design

`agent-otel` exposes one bootstrap path for traces, metrics, logs, GenAI
instruments, optional Datadog defaults, and optional local `lotel` dev-sink
export.

Verified on 2026-05-25 against current OTel SDK configuration docs and current
advisor/local-symphony bootstrap code:

- OTel OTLP exporter configuration supports general and per-signal endpoint,
  headers, protocol, timeout, compression, certificate, and client key variables.
- Per-signal environment variables should override general OTLP variables.
- Advisor currently ships trace+metric only, with `InitServe`, `InitCLI`, and
  `InitDisabled` wrappers.
- Symphony currently ships trace+metric+log providers, resource construction,
  global install, and reverse-order shutdown, but its env precedence is
  general-over-per-signal and must be corrected for `agent-otel`.

## Public Surface

```go
type Options struct {
	Enabled bool

	ServiceName    string
	ServiceVersion string
	Environment    string
	ResourceAttrs   []attribute.KeyValue

	SkipGlobalInstall bool
	DialTimeout       time.Duration
	BatchTimeout      time.Duration
	ExportTimeout     time.Duration

	TraceExporter  ExporterConfig
	MetricExporter ExporterConfig
	LogExporter    ExporterConfig

	DatadogPreset *DatadogPreset
	DevSink       *DevSinkConfig
	Instruments   InstrumentOptions
}

type ExporterConfig struct {
	Endpoint string
	Protocol string // grpc or http/protobuf
	Headers  map[string]string
	Insecure bool
	Timeout  time.Duration
}

type Providers struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider
	Tracer         trace.Tracer
	Meter          metric.Meter
	Logger         log.Logger
	Instruments    *Instruments
	Resource       *resource.Resource
}

func Init(ctx context.Context, opts Options) (*Providers, *Shutdown, error)
```

`Enabled=false` returns non-nil no-op providers, no-op instruments, and a no-op
shutdown without touching exporters.

## Environment Resolution

`agent-otel` follows OTel-spec precedence:

```text
explicit Options field
> per-signal OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_*
> general OTEL_EXPORTER_OTLP_*
> default
```

Endpoint/header/protocol/insecure resolution must be signal-specific.

| Setting | Trace env | Metric env | Log env | General env | Default |
| --- | --- | --- | --- | --- | --- |
| endpoint | `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` | `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://localhost:4318` for HTTP or `localhost:4317` for gRPC |
| headers | `OTEL_EXPORTER_OTLP_TRACES_HEADERS` | `OTEL_EXPORTER_OTLP_METRICS_HEADERS` | `OTEL_EXPORTER_OTLP_LOGS_HEADERS` | `OTEL_EXPORTER_OTLP_HEADERS` | empty |
| protocol | `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` | `OTEL_EXPORTER_OTLP_METRICS_PROTOCOL` | `OTEL_EXPORTER_OTLP_LOGS_PROTOCOL` | `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc` |
| timeout | `OTEL_EXPORTER_OTLP_TRACES_TIMEOUT` | `OTEL_EXPORTER_OTLP_METRICS_TIMEOUT` | `OTEL_EXPORTER_OTLP_LOGS_TIMEOUT` | `OTEL_EXPORTER_OTLP_TIMEOUT` | `ExportTimeout` |

Headers parse the OTel `k=v,k2=v2` form with URL decoding. Header values are
secret material: never log them.

## Endpoint and Protocol Decisions

| Input | Protocol | Exporter option behavior |
| --- | --- | --- |
| `grpc`, endpoint `localhost:4317` | gRPC | pass bare endpoint to `otlp*grpc.WithEndpoint`; use `WithInsecure` when plaintext |
| `grpc`, endpoint `http://host:4317` | gRPC | strip `http://`, set insecure true |
| `grpc`, endpoint `https://host:4317` | gRPC | strip `https://`, set insecure false |
| `http/protobuf`, endpoint `https://host:4318` | HTTP | pass URL to `otlp*http.WithEndpointURL` or equivalent URL-aware option |
| `http/protobuf`, endpoint `host:4318` | HTTP | normalize to `http://host:4318` when insecure, otherwise `https://host:4318` |
| unsupported protocol | none | return `ConfigValidationError` before building exporters |

For gRPC, the endpoint option should not receive a URL scheme. For HTTP, prefer
the URL-aware exporter option so paths like `/v1/traces` survive.

Validate env-supplied endpoints for malformed schemes, embedded whitespace,
empty host, and invalid port. Programmatic `Options` may return validation
errors too; unlike symphony, the shared library should fail early for both env
and API input because it is a reusable module.

## Exporter Construction and Fail-Open

Default mode builds exporters in this order:

1. Trace exporter and provider.
2. Metric exporter and provider.
3. Log exporter and provider.
4. Instruments from the meter.
5. Global install unless skipped.

If the primary exporter path fails during initialization:

- Return no-op providers and no-op instruments.
- Return a no-op shutdown.
- Return `nil` error for network/dial/exporter availability failures.
- Return a non-nil error for programmer/configuration errors such as invalid
  protocol, invalid endpoint syntax, resource construction failure, or duplicate
  instrument registration.
- Clean up any partially initialized providers before returning.

This preserves advisor's fail-open startup posture while still making operator
typos explicit.

## Datadog Preset

`WithDatadogPreset()` applies only when explicitly requested:

- Prefer trace OTLP/HTTP with `http/protobuf`.
- Add `dd-api-key` from `DD_API_KEY`.
- Add `dd-otlp-source=llmobs`.
- Respect explicit options and standard OTel env vars over preset defaults.
- Do not enable payload capture.
- Do not enable OpenLLMetry compatibility.

The preset must not prevent a caller from routing through a local Collector or
Datadog Agent. If the caller supplies an endpoint, use it.

## Dev Sink

`WithDevSink(lotelEndpoint)` sends telemetry to the primary configured backend
and a local `lotel` instance.

Rules:

- Dev sink is opt-in.
- Dev sink defaults to fail-open. A missing or unreachable local `lotel` must
  never disable the primary exporter path.
- Dev sink should support gRPC endpoint `localhost:4317` and HTTP endpoint
  `http://localhost:4318`.
- Primary exporter failures still follow the primary fail-open rules.
- Dev sink exporter failures are logged once without secrets and then ignored.
- Shutdown/ForceFlush should include dev-sink providers when they were created.

SDK composition decision:

- Prefer separate exporter instances feeding one provider per signal through
  fan-out processors/readers where the Go SDK supports it cleanly.
- If metric reader fan-out is awkward, create a small internal fan-out exporter
  for metrics rather than exposing two meter providers to callers.
- The caller still receives one `TracerProvider`, one `MeterProvider`, and one
  `LoggerProvider`.

## ForceFlush and Shutdown

Expose explicit lifecycle methods:

```go
type Shutdown struct {
	// unexported closures
}

func (s *Shutdown) ForceFlush(ctx context.Context) error
func (s *Shutdown) Shutdown(ctx context.Context) error
```

Order:

1. `ForceFlush`: logs, metrics, traces.
2. `Shutdown`: logs, metrics, traces.

Rationale: log records may carry span context, and metrics may be emitted by
span-ending paths. Keeping traces alive until last preserves correlation while
other signals drain.

Both methods are idempotent. They join errors from all components. Callers must
pass bounded contexts for CLI/single-shot modes.

Advisor compatibility wrappers:

- `InitServe` maps to default batch/export timeouts.
- `InitCLI` maps to short batch/export timeouts.
- `InitDisabled` maps to `Options{Enabled:false}`.

## Required Tests

Environment resolution:

- Per-signal endpoint beats general endpoint for traces, metrics, and logs.
- Explicit `Options.TraceExporter.Endpoint` beats env vars.
- Per-signal headers beat general headers.
- Header parsing URL-decodes values and never logs secrets.
- Per-signal protocol beats general protocol.
- `http://` and `https://` endpoint schemes set TLS/insecure behavior
  correctly for gRPC.
- HTTP exporter preserves full endpoint URL path such as `/v1/traces`.
- Malformed env and malformed Options endpoints return typed validation errors.

Exporter behavior:

- `Enabled=false` returns non-nil no-op providers, instruments, and shutdown.
- Trace exporter availability failure falls back to no-op without error.
- Metric exporter availability failure cleans up trace provider then falls back
  to no-op without error.
- Log exporter availability failure cleans up trace/metric providers then falls
  back to no-op without error.
- Invalid protocol returns a non-nil error.
- Resource merge failure returns a non-nil error.
- `SkipGlobalInstall=true` avoids global OTel mutations.
- `SkipGlobalInstall=false` installs tracer, meter, logger, and W3C
  trace-context/baggage propagators.

Datadog preset:

- Adds `dd-api-key` and `dd-otlp-source=llmobs`.
- Selects trace HTTP/protobuf defaults.
- Does not override explicit endpoint or protocol.
- Does not enable payload capture or OpenLLMetry compatibility.

Dev sink:

- Primary and dev exporters both receive a span/metric/log when both are up.
- Missing dev sink does not prevent primary export.
- Dev-sink failure logs once without endpoint headers.
- Shutdown drains dev-sink exporters that were successfully created.

Lifecycle:

- ForceFlush order is logs, metrics, traces.
- Shutdown order is logs, metrics, traces.
- Methods are idempotent and return the same joined error on repeated calls.
- CLI wrapper flush/shutdown completes within caller deadline against an
  unreachable endpoint.

## References

- https://opentelemetry.io/docs/languages/sdk-configuration/otlp-exporter/
- https://docs.datadoghq.com/llm_observability/instrumentation/otel_instrumentation/
- `/home/infra-admin/git/advisor/internal/telemetry/cli_init.go`
- `/home/infra-admin/git/local-symphony/internal/obs/config.go`
- `/home/infra-admin/git/local-symphony/internal/obs/otel.go`
