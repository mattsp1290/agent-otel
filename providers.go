package agentotel

import (
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

const instrumentationName = "github.com/mattsp1290/agent-otel"

// Providers groups the OpenTelemetry providers and package instruments created by Init.
type Providers struct {
	TracerProvider trace.TracerProvider
	MeterProvider  metric.MeterProvider
	LoggerProvider log.LoggerProvider

	Tracer trace.Tracer
	Meter  metric.Meter
	Logger log.Logger

	Instruments *Instruments
	Resource    *resource.Resource

	openLLMetryCompat *OpenLLMetryCompatOptions
	payloadCapture    *PayloadCaptureOptions
}

// Instruments groups package-owned telemetry instruments.
type Instruments struct {
	ModelLatency       metric.Float64Histogram
	UsageInputTokens   metric.Int64Histogram
	UsageOutputTokens  metric.Int64Histogram
	ErrorsByProvider   metric.Int64Counter
	FallbackEngaged    metric.Int64Counter
	cardinality        *CardinalityValidator
	tokenUsageRecorder metric.Int64Histogram
}

func newInstruments(meter metric.Meter, opts InstrumentOptions) (*Instruments, error) {
	mode := opts.CardinalityMode
	if mode == 0 {
		mode = CardinalityLogAndDrop
	}
	modelLatency, err := meter.Float64Histogram(MetricGenAIClientOperationDuration,
		metric.WithUnit("s"),
		metric.WithDescription("Duration of a GenAI client operation."),
	)
	if err != nil {
		return nil, err
	}
	tokenUsage, err := meter.Int64Histogram(MetricGenAIClientTokenUsage,
		metric.WithUnit("{token}"),
		metric.WithDescription("Token usage by token type for a GenAI client operation."),
	)
	if err != nil {
		return nil, err
	}
	errorsByProvider, err := meter.Int64Counter(MetricAgentOTelProviderErrors,
		metric.WithUnit("{error}"),
		metric.WithDescription("Provider errors by bounded provider and error type."),
	)
	if err != nil {
		return nil, err
	}
	fallbackEngaged, err := meter.Int64Counter(MetricAgentOTelFallbackEngaged,
		metric.WithUnit("{event}"),
		metric.WithDescription("Provider fallback activations by bounded provider route."),
	)
	if err != nil {
		return nil, err
	}
	return &Instruments{
		ModelLatency:       modelLatency,
		UsageInputTokens:   tokenUsage,
		UsageOutputTokens:  tokenUsage,
		ErrorsByProvider:   errorsByProvider,
		FallbackEngaged:    fallbackEngaged,
		cardinality:        NewCardinalityValidator(WithCardinalityMode(mode)),
		tokenUsageRecorder: tokenUsage,
	}, nil
}
