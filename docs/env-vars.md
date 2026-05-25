# Environment Variables

`agent-otel` follows OpenTelemetry OTLP exporter precedence:

```text
explicit Options field
> per-signal OTEL_EXPORTER_OTLP_{TRACES,METRICS,LOGS}_*
> general OTEL_EXPORTER_OTLP_*
> default
```

Per-signal values win over general values. This applies independently to
traces, metrics, and logs.

## Endpoints

| Signal | Per-signal env | General fallback | Default |
| --- | --- | --- | --- |
| traces | `OTEL_EXPORTER_OTLP_TRACES_ENDPOINT` | `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` for gRPC or `http://localhost:4318` for HTTP |
| metrics | `OTEL_EXPORTER_OTLP_METRICS_ENDPOINT` | `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` for gRPC or `http://localhost:4318` for HTTP |
| logs | `OTEL_EXPORTER_OTLP_LOGS_ENDPOINT` | `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` for gRPC or `http://localhost:4318` for HTTP |

Example: split traces and metrics across collectors while logs use the general
collector:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=http://collector-general:4318
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://collector-traces:4318/v1/traces
export OTEL_EXPORTER_OTLP_METRICS_ENDPOINT=http://collector-metrics:4318/v1/metrics
```

Resolution:

- traces -> `collector-traces`
- metrics -> `collector-metrics`
- logs -> `collector-general`

Example: one local collector for everything:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_INSECURE=true
```

## Protocol

| Signal | Per-signal env | General fallback | Supported values |
| --- | --- | --- | --- |
| traces | `OTEL_EXPORTER_OTLP_TRACES_PROTOCOL` | `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc`, `http/protobuf` |
| metrics | `OTEL_EXPORTER_OTLP_METRICS_PROTOCOL` | `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc`, `http/protobuf` |
| logs | `OTEL_EXPORTER_OTLP_LOGS_PROTOCOL` | `OTEL_EXPORTER_OTLP_PROTOCOL` | `grpc`, `http/protobuf` |

gRPC example:

```bash
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_INSECURE=true
```

HTTP/protobuf example:

```bash
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
```

Mixed protocol example:

```bash
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
export OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=http://collector:4318/v1/traces
```

Resolution:

- traces use OTLP/HTTP to `/v1/traces`
- metrics and logs use OTLP/gRPC to `localhost:4317`

## Headers

| Signal | Per-signal env | General fallback |
| --- | --- | --- |
| traces | `OTEL_EXPORTER_OTLP_TRACES_HEADERS` | `OTEL_EXPORTER_OTLP_HEADERS` |
| metrics | `OTEL_EXPORTER_OTLP_METRICS_HEADERS` | `OTEL_EXPORTER_OTLP_HEADERS` |
| logs | `OTEL_EXPORTER_OTLP_LOGS_HEADERS` | `OTEL_EXPORTER_OTLP_HEADERS` |

Headers use OTel's comma-separated `key=value` format. URL-encode literal
commas or equals signs inside values.

```bash
export OTEL_EXPORTER_OTLP_HEADERS='authorization=Bearer%20abc123,x-team=agent'
export OTEL_EXPORTER_OTLP_TRACES_HEADERS='authorization=Bearer%20trace-only'
```

Resolution:

- traces use `authorization=Bearer trace-only`
- metrics and logs use `authorization=Bearer abc123,x-team=agent`

Header values are secret material. `agent-otel` must not log them.

## TLS and Insecure Mode

For gRPC endpoints:

| Endpoint | Insecure setting | Result |
| --- | --- | --- |
| `http://collector:4317` | any | plaintext, scheme stripped before gRPC exporter |
| `https://collector:4317` | any | TLS, scheme stripped before gRPC exporter |
| `collector:4317` | `OTEL_EXPORTER_OTLP_INSECURE=true` | plaintext |
| `collector:4317` | unset or false | TLS |

For HTTP/protobuf endpoints, use full URLs:

```bash
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=https://collector.example.com:4318
```

## Timeouts

| Signal | Per-signal env | General fallback |
| --- | --- | --- |
| traces | `OTEL_EXPORTER_OTLP_TRACES_TIMEOUT` | `OTEL_EXPORTER_OTLP_TIMEOUT` |
| metrics | `OTEL_EXPORTER_OTLP_METRICS_TIMEOUT` | `OTEL_EXPORTER_OTLP_TIMEOUT` |
| logs | `OTEL_EXPORTER_OTLP_LOGS_TIMEOUT` | `OTEL_EXPORTER_OTLP_TIMEOUT` |

Callers should still pass bounded contexts to `ForceFlush` and `Shutdown`,
especially in CLI processes.

## Datadog Preset

`WithDatadogPreset()` adds Datadog-friendly defaults without overriding explicit
operator routing.

Direct Datadog intake example:

```bash
export DD_API_KEY=...
export OTEL_EXPORTER_OTLP_TRACES_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=https://otlp.datadoghq.com/v1/traces
```

The preset adds:

- `dd-api-key` from `DD_API_KEY`
- `dd-otlp-source=llmobs`

Collector or Datadog Agent example:

```bash
export DD_API_KEY=...
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
```

The preset must not enable prompt payload capture or OpenLLMetry compatibility.

## Local lotel Dev Sink

`WithDevSink(lotelEndpoint)` duplicates telemetry to a local `lotel` instance
while preserving the primary exporter path.

gRPC lotel:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=https://primary-collector.example.com:4317
# application option:
# agentotel.WithDevSink("localhost:4317")
```

HTTP lotel:

```bash
export OTEL_EXPORTER_OTLP_PROTOCOL=http/protobuf
export OTEL_EXPORTER_OTLP_ENDPOINT=https://primary-collector.example.com/v1/traces
# application option:
# agentotel.WithDevSink("http://localhost:4318")
```

Dev sink behavior:

- Missing local `lotel` fails open.
- Primary exporter failures follow normal fail-open rules.
- Dev-sink export, flush, and shutdown errors are ignored after the primary
  exporter has been called.
- `ForceFlush` and `Shutdown` drain the dev sink when it was created.

## Validation

`agent-otel` should reject malformed endpoint configuration before exporters
are built:

- embedded whitespace
- unsupported scheme
- empty host
- invalid port
- unsupported protocol

Use a full URL for HTTP/protobuf and a bare `host:port` or `http(s)://host:port`
for gRPC.
