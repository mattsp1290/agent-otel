# Usage Boundary

`agent-otel` records normalized token usage; it does not extract usage from
provider SDKs, Eino messages, streams, or graph outputs in the core package.

Core callers pass this value:

```go
type Usage struct {
	InputTokens  int64
	OutputTokens int64
	Available    bool
}
```

`InputTokens` maps to prompt/input tokens. `OutputTokens` maps to
completion/response tokens.

## Available Semantics

`Available=false` means the upstream provider or framework did not report real
usage for this operation. In that case:

- `InputTokens` and `OutputTokens` must be ignored by callers and dashboards.
- Span attributes should include a low-cardinality availability marker, such as
  `agent_otel.usage.available=false`, so missing usage is visible.
- Token counters and histograms must not record zero token values as if they
  were real usage.
- The model-call span may still include provider, model, operation, latency,
  and error attributes.

`Available=true` means both token fields are authoritative for this operation
after provider/framework normalization. Negative values are invalid at the
`agent-otel` boundary; callers should clamp or reject provider-specific bad
values before constructing `Usage`.

## Core API

Core should expose helpers that consume normalized usage:

```go
func RecordUsage(ctx context.Context, usage Usage, labels ModelLabels)
func SetUsageSpanAttributes(span trace.Span, usage Usage)
```

The exact helper names can be finalized during implementation, but the contract
is fixed:

- Helpers accept `agentotel.Usage`, not `schema.Message`, `schema.TokenUsage`,
  provider response structs, or stream chunks.
- Helpers emit `gen_ai.usage.input_tokens` and
  `gen_ai.usage.output_tokens` only when `Available=true`.
- Helpers emit the local availability marker for both available and unavailable
  cases.
- Helpers share the same label/cardinality checks as the other model-call
  recorders.

The core package must not import `github.com/cloudwego/eino`.

## Advisor Example

Advisor already has the target normalized shape in
`internal/advisor/provider.go`:

```go
type Usage struct {
	InputTokens  int
	OutputTokens int
	Available    bool
}
```

Advisor provider implementations call Eino ChatModel APIs and convert
`schema.Message.ResponseMeta.Usage` through `extractUsage`. That extraction
belongs in advisor because the provider interface owns the single-call
contract:

```go
func extractUsage(msg *schema.Message) Usage {
	if msg != nil && msg.ResponseMeta != nil && msg.ResponseMeta.Usage != nil {
		return Usage{
			InputTokens:  msg.ResponseMeta.Usage.PromptTokens,
			OutputTokens: msg.ResponseMeta.Usage.CompletionTokens,
			Available:    true,
		}
	}
	return Usage{Available: false}
}
```

During migration, `advisor/internal/advisor/core/advise.go` should convert this
value to `agentotel.Usage` immediately before recording telemetry:

```go
agentUsage := agentotel.Usage{
	InputTokens:  int64(usage.InputTokens),
	OutputTokens: int64(usage.OutputTokens),
	Available:    usage.Available,
}
agentotel.RecordUsage(ctx, agentUsage, labels)
```

The advisor `Provider.Advise` interface should continue to return advisor's
domain `Usage` unless a separate advisor migration decides to expose
`agentotel.Usage` directly. The extraction from `schema.Message` should not
move into `agent-otel`.

## Symphony Eino Graph Example

Symphony currently uses `core.TokenCounters`:

```go
type TokenCounters struct {
	PromptTokens     uint64 `json:"prompt_tokens"`
	CompletionTokens uint64 `json:"completion_tokens"`
}
```

The Eino graph extracts usage in
`internal/worker/agent/graph.go::outputFromMessage` from
`schema.Message.ResponseMeta.Usage`, clamps negative provider values to zero,
and returns per-turn token counts:

```go
if u := msg.ResponseMeta.Usage; u != nil {
	out.Tokens = core.TokenCounters{
		PromptTokens:     nonNegativeUint64(u.PromptTokens),
		CompletionTokens: nonNegativeUint64(u.CompletionTokens),
	}
}
```

The worker then accumulates those per-turn values with
`core.TokenCounters.Add` before emitting terminal run events. That accumulation
is a symphony runtime concern, not an `agent-otel` concern. Symphony should
convert the final per-operation or per-turn value at the telemetry emission
site:

```go
agentUsage := agentotel.Usage{
	InputTokens:  int64(tokens.PromptTokens),
	OutputTokens: int64(tokens.CompletionTokens),
	Available:    usageWasReported,
}
agentotel.RecordUsage(ctx, agentUsage, labels)
```

Symphony currently represents missing usage as zero-value `TokenCounters`
without an explicit `Available` bit. Its migration must preserve a boolean at
the extraction boundary so "provider reported 0 tokens" and "provider did not
report usage" do not collapse into the same telemetry. The natural place is
the graph `Output` or an adjacent worker-local usage struct.

## Eino Boundary

Do not add an `agentotel/eino` adapter for the initial implementation.

Reasons:

- Advisor and symphony use Eino at different layers. Advisor's public provider
  interface already returns normalized usage, while symphony extracts usage
  inside a graph collect lambda and accumulates it in worker runtime state.
- Eino provider adapters expose usage through `schema.Message.ResponseMeta`,
  but stream semantics differ. Symphony explicitly depends on
  `schema.ConcatMessages` MAX-merging usage for the current Ollama streaming
  shape; that is a consumer-specific invariant and must stay close to the
  graph tests that pin it.
- A core dependency on Eino would make `agent-otel` less useful for callers
  that use direct provider SDKs or other agent frameworks.

An optional `agentotel/eino` subpackage may be reconsidered only after both
consumer migrations land and show duplicated, stable extraction code. If added
later, it must be a subpackage with its own Eino dependency and must return
`agentotel.Usage`; core still must not import Eino.

## Tests Required By Implementation

Implementation tests should cover:

- `Available=false` records the availability marker and no token metric values.
- `Available=true` records input and output token metrics and span attributes.
- Negative token counts at the `agent-otel` boundary are rejected or normalized
  according to the implementation decision; they must not silently underflow.
- Advisor migration tests prove `Provider.Advise` usage maps to
  `agentotel.Usage`.
- Symphony migration tests prove graph-extracted Eino usage maps to
  `agentotel.Usage` and missing usage remains distinguishable from zero usage.
- Core package import tests or lint checks prevent an Eino import from entering
  the root package.
