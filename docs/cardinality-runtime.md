# Cardinality Runtime and Lint Enforcement

`agent-otel` enforces metric cardinality before attributes reach OTel
instruments. Symphony's current `internal/obs/cardinality.go` is the right
source pattern, but it was test-only: callers could still invoke raw OTel
instruments with arbitrary `metric.WithAttributes(...)`. The shared module
makes safe wrapper recorders the normal public API.

## Budget Source

Maintain one allowlist table keyed by canonical metric name:

```go
type MetricSpec struct {
	Name        string
	Unit        string
	Description string
	AllowedKeys []string
}
```

The built-in model-call budget covers:

| Metric | Allowed keys |
| --- | --- |
| `gen_ai.client.operation.duration` | `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.operation.name`, `error.type` |
| `gen_ai.client.token.usage` | `gen_ai.provider.name`, `gen_ai.request.model`, `gen_ai.operation.name`, `gen_ai.token.type` |
| `agent_otel.provider.errors` | `gen_ai.provider.name`, `error.type` |
| `agent_otel.fallback.engaged` | `gen_ai.provider.name`, `agent_otel.provider.from`, `agent_otel.provider.to` |

The exact GenAI metric names are pinned in `genai_attrs.go` and summarized in
`docs/genai-mapping.md`.

Globally prohibited metric keys:

- `issue.id`
- `session.id`
- `thread.id`
- `turn.id`
- `run_attempt.id`
- keys with prefix `tool.input/`
- prompt/completion payload fields, including `gen_ai.input.messages`,
  `gen_ai.output.messages`, `gen_ai.system_instructions`, and
  `gen_ai.tool.definitions`

These keys may be valid span or event attributes, but not metric labels.

## Public Recording API

Expose typed recorders instead of raw OTel instruments:

```go
type Instruments struct {
	ModelLatency       metric.Float64Histogram
	UsageInputTokens   metric.Int64Histogram
	UsageOutputTokens  metric.Int64Histogram
	ErrorsByProvider   metric.Int64Counter
	FallbackEngaged    metric.Int64Counter
}

type ModelMetricLabels struct {
	OperationName string
	ProviderName  string
	RequestModel  string
	ErrorType     string
}

type ProviderErrorLabels struct {
	ProviderName string
	ErrorType    string
}

type Fallback struct {
	ProviderName string
	FromProvider string
	ToProvider   string
	Attributes   []attribute.KeyValue
}

func (i *Instruments) RecordModelLatency(ctx context.Context, seconds float64, labels ModelMetricLabels) error
func (i *Instruments) RecordUsage(ctx context.Context, usage Usage, labels ModelMetricLabels) error
func (i *Instruments) RecordProviderError(ctx context.Context, labels ProviderErrorLabels) error
func (i *Instruments) RecordFallbackEngaged(ctx context.Context, fallback Fallback) error
```

Wrapper methods construct the OTel attributes internally, run the cardinality
filter, and then call the underlying OTel instrument.

If custom project metrics are needed later, add a registration API that requires
an allowlist, such as:

```go
func (i *Instruments) RegisterInt64Counter(spec MetricSpec) (*Int64CounterRecorder, error)
func (r *Int64CounterRecorder) Add(ctx context.Context, value int64, attrs ...attribute.KeyValue)
```

Registration fails when the metric name is empty, duplicated, or omits an
allowlist entry. Runtime recording through a registered custom recorder uses
the same filter as built-in recorders.

## Runtime Behavior

Runtime validation is enabled by default for wrapper recorders. The default
policy is log-and-drop the bad label, then record the measurement with the
remaining allowed labels.

Decision details:

- Allowed key with any value: keep the label.
- Prohibited key or prohibited prefix: drop that label.
- Unknown key for a known metric: drop that label.
- Unknown metric through `CheckAllowedLabels`: return an error.
- Unknown metric through custom runtime recording: impossible when using the
  registration API; if internal state is corrupted, drop the whole measurement
  and log once.
- Empty allowed value: keep it only when the implementation documents the value
  as a bounded enum or stable identifier; otherwise drop it.

Dropping one bad label should not drop the whole data point for known metrics.
The point still carries useful bounded dimensions, and silently losing the
measurement would make debugging harder.

## Logging Policy

Log each cardinality violation once per process per `(metric, key, reason)`.
Do not log every call.

Reasons:

- A bad hot-path label can occur on every model request; every-call logging
  can create the same cardinality and cost failure in logs.
- Once-per-key logging still gives operators and tests a visible signal.
- Unit tests can inject a fresh violation logger to assert the exact first
  warning without depending on process-global state.

Violation logs must not include attribute values. They should include:

- metric name
- attribute key
- reason: `prohibited`, `unknown_key`, `unknown_metric`, or `invalid_value`
- action: `dropped_label` or `dropped_measurement`

## Strict Mode

Provide a strict validation option for tests and development:

```go
type CardinalityMode int

const (
	CardinalityLogAndDrop CardinalityMode = iota
	CardinalityStrict
	CardinalityDisabled
)
```

- `CardinalityLogAndDrop`: default production mode.
- `CardinalityStrict`: wrapper recorders return or record errors in testable
  form, and custom recorders refuse measurements containing bad labels.
- `CardinalityDisabled`: only for benchmarks or emergency operator override;
  tests must prove this mode is never selected by default presets.

The built-in no-error recorder methods can expose strict failures through an
optional error sink on `Options`, or strict variants can return errors:

```go
func (i *Instruments) ValidateModelLatency(labels ModelLabels) error
func (r *Int64CounterRecorder) AddStrict(ctx context.Context, value int64, attrs ...attribute.KeyValue) error
```

## Test and Lint Helpers

Keep exported helpers similar to symphony's current API:

```go
func AllowedKeys(metricName string) ([]string, bool)
func ProhibitedKeys() []string
func ProhibitedPrefixes() []string
func IsProhibitedKey(key string) bool
func CheckAllowedLabels(metricName string, keys ...string) error
```

Required unit tests:

- Every built-in metric has a `MetricSpec` allowlist entry.
- `AllowedKeys`, `ProhibitedKeys`, and `ProhibitedPrefixes` return defensive
  copies.
- `CheckAllowedLabels` accepts every documented happy-path key set.
- `CheckAllowedLabels` rejects prohibited keys for every metric.
- `CheckAllowedLabels` rejects keys outside the metric allowlist.
- `CheckAllowedLabels` rejects unknown metric names when keys are supplied.
- Runtime filtering keeps allowed labels and drops prohibited/unknown labels.
- Runtime filtering logs once per `(metric, key, reason)`, not every call.
- Unknown custom metric state drops the whole measurement and logs once.
- Strict mode surfaces violations without exporting prohibited labels.
- Disabled mode bypasses filtering only when explicitly configured.

Required lint/static tests:

- A repository scan fails when raw OTel `metric.WithAttributes(...)` uses a
  prohibited key in a metric context.
- A repository scan fails when built-in metric emit sites bypass wrapper
  recorder methods.
- Span/event attributes are exempt from metric-label prohibitions.
- The suppression comment, if needed, must be same-line and narrow:
  `//nolint:cardinality-budget`.

Use `go/parser` or `golang.org/x/tools/go/analysis` for the long-term lint
shape. A regex walker is acceptable only as a temporary test helper while the
module skeleton is still small.
