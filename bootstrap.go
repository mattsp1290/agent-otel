package agentotel

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/log/global"
	lognoop "go.opentelemetry.io/otel/log/noop"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const defaultServiceName = "agent-otel"

// Init initializes agent OpenTelemetry providers.
func Init(ctx context.Context, opts Options) (*Providers, *Shutdown, error) {
	res, err := buildResource(ctx, opts)
	if err != nil {
		return nil, nil, err
	}

	providers := noopProviders(opts, res)
	shutdown := newShutdown(nil, nil)

	if !opts.SkipGlobalInstall {
		installGlobalProviders(providers)
	}

	return providers, shutdown, nil
}

func noopProviders(opts Options, res *resource.Resource) *Providers {
	tracerProvider := tracenoop.NewTracerProvider()
	meterProvider := metricnoop.NewMeterProvider()
	loggerProvider := lognoop.NewLoggerProvider()

	return &Providers{
		TracerProvider: tracerProvider,
		MeterProvider:  meterProvider,
		LoggerProvider: loggerProvider,
		Tracer:         tracerProvider.Tracer(instrumentationName),
		Meter:          meterProvider.Meter(instrumentationName),
		Logger:         loggerProvider.Logger(instrumentationName),
		Instruments:    newInstruments(opts.Instruments),
		Resource:       res,
	}
}

func installGlobalProviders(providers *Providers) {
	otel.SetTracerProvider(providers.TracerProvider)
	otel.SetMeterProvider(providers.MeterProvider)
	global.SetLoggerProvider(providers.LoggerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
}

func buildResource(_ context.Context, opts Options) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{semconv.ServiceName(serviceName(opts))}
	if opts.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(opts.ServiceVersion))
	}
	if opts.Environment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentName(opts.Environment))
	}
	attrs = append(attrs, opts.ResourceAttrs...)

	return resource.Merge(resource.Default(), resource.NewSchemaless(attrs...))
}

func serviceName(opts Options) string {
	if opts.ServiceName != "" {
		return opts.ServiceName
	}
	return defaultServiceName
}
