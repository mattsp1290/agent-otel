package agentotel

import (
	"context"
	"strings"

	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func composeTraceExporter(ctx context.Context, primary sdktrace.SpanExporter, primaryCfg resolvedExporterConfig, opts Options) sdktrace.SpanExporter {
	devCfg, ok := resolveDevSinkExporter(primaryCfg, opts)
	if !ok {
		return primary
	}
	if err := dialEndpoint(ctx, devCfg, opts.DialTimeout); err != nil {
		return primary
	}
	dev, err := newTraceExporter(ctx, devCfg)
	if err != nil {
		return primary
	}
	return multiTraceExporter{primary: primary, dev: dev}
}

func composeMetricExporter(ctx context.Context, primary sdkmetric.Exporter, primaryCfg resolvedExporterConfig, opts Options) sdkmetric.Exporter {
	devCfg, ok := resolveDevSinkExporter(primaryCfg, opts)
	if !ok {
		return primary
	}
	if err := dialEndpoint(ctx, devCfg, opts.DialTimeout); err != nil {
		return primary
	}
	dev, err := newMetricExporter(ctx, devCfg)
	if err != nil {
		return primary
	}
	return multiMetricExporter{primary: primary, dev: dev}
}

func composeLogExporter(ctx context.Context, primary sdklog.Exporter, primaryCfg resolvedExporterConfig, opts Options) sdklog.Exporter {
	devCfg, ok := resolveDevSinkExporter(primaryCfg, opts)
	if !ok {
		return primary
	}
	if err := dialEndpoint(ctx, devCfg, opts.DialTimeout); err != nil {
		return primary
	}
	dev, err := newLogExporter(ctx, devCfg)
	if err != nil {
		return primary
	}
	return multiLogExporter{primary: primary, dev: dev}
}

func resolveDevSinkExporter(primary resolvedExporterConfig, opts Options) (resolvedExporterConfig, bool) {
	if opts.DevSink == nil || !opts.DevSink.Enabled {
		return resolvedExporterConfig{}, false
	}
	endpoint := strings.TrimSpace(opts.DevSink.Endpoint)
	if endpoint == "" {
		endpoint = defaultGRPCEndpoint
	}
	protocol := ProtocolGRPC
	if strings.HasPrefix(strings.ToLower(endpoint), "http://") || strings.HasPrefix(strings.ToLower(endpoint), "https://") {
		protocol = ProtocolHTTPProtobuf
	}
	normalizedEndpoint, insecure, err := normalizeEndpointForProtocol("Options.DevSink.Endpoint", opts.DevSink.Endpoint, endpoint, protocol, true)
	if err != nil {
		return resolvedExporterConfig{}, false
	}
	return resolvedExporterConfig{
		Endpoint: normalizedEndpoint,
		Protocol: protocol,
		Insecure: insecure,
		Timeout:  primary.Timeout,
	}, true
}

type multiTraceExporter struct {
	primary sdktrace.SpanExporter
	dev     sdktrace.SpanExporter
}

func (e multiTraceExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	err := e.primary.ExportSpans(ctx, spans)
	_ = e.dev.ExportSpans(ctx, spans)
	return err
}

func (e multiTraceExporter) Shutdown(ctx context.Context) error {
	err := e.primary.Shutdown(ctx)
	_ = e.dev.Shutdown(ctx)
	return err
}

type multiMetricExporter struct {
	primary sdkmetric.Exporter
	dev     sdkmetric.Exporter
}

func (e multiMetricExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return e.primary.Temporality(kind)
}

func (e multiMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return e.primary.Aggregation(kind)
}

func (e multiMetricExporter) Export(ctx context.Context, data *metricdata.ResourceMetrics) error {
	err := e.primary.Export(ctx, data)
	_ = e.dev.Export(ctx, data)
	return err
}

func (e multiMetricExporter) ForceFlush(ctx context.Context) error {
	err := e.primary.ForceFlush(ctx)
	_ = e.dev.ForceFlush(ctx)
	return err
}

func (e multiMetricExporter) Shutdown(ctx context.Context) error {
	err := e.primary.Shutdown(ctx)
	_ = e.dev.Shutdown(ctx)
	return err
}

type multiLogExporter struct {
	primary sdklog.Exporter
	dev     sdklog.Exporter
}

func (e multiLogExporter) Export(ctx context.Context, records []sdklog.Record) error {
	err := e.primary.Export(ctx, records)
	_ = e.dev.Export(ctx, records)
	return err
}

func (e multiLogExporter) ForceFlush(ctx context.Context) error {
	err := e.primary.ForceFlush(ctx)
	_ = e.dev.ForceFlush(ctx)
	return err
}

func (e multiLogExporter) Shutdown(ctx context.Context) error {
	err := e.primary.Shutdown(ctx)
	_ = e.dev.Shutdown(ctx)
	return err
}
