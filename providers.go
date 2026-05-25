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
}

// Instruments groups package-owned telemetry instruments.
type Instruments struct {
	cardinality *CardinalityValidator
}

func newInstruments(opts InstrumentOptions) *Instruments {
	mode := opts.CardinalityMode
	if mode == 0 {
		mode = CardinalityLogAndDrop
	}
	return &Instruments{
		cardinality: NewCardinalityValidator(WithCardinalityMode(mode)),
	}
}
