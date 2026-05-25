# agent-otel

`agent-otel` is a small Go library for shared OpenTelemetry setup and
agent-oriented telemetry helpers.

The first planned consumers are:

- [`advisor`](https://github.com/mattsp1290/advisor)
- [`local-symphony`](https://github.com/mattsp1290/local-symphony)

The initial module is intentionally behavior-free. Follow-up beads add the
OTel dependency pins, bootstrap API, GenAI semantic-convention mapping,
redaction controls, metric-cardinality guards, and consumer migrations.

## Development

```bash
go test ./...
```
