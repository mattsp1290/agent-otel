package agentotel

import (
	"context"
	"errors"
	"net"
	"net/url"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

const defaultDialTimeout = 250 * time.Millisecond

func buildOTLPProviders(ctx context.Context, cfg resolvedConfig, opts Options, res *resource.Resource) (*Providers, *Shutdown, error) {
	if !exporterEndpointsReachable(ctx, cfg, opts.DialTimeout) {
		providers := noopProviders(opts, res)
		return providers, newShutdown(nil, nil), nil
	}

	traceExporter, err := newTraceExporter(ctx, cfg.Traces)
	if err != nil {
		return noopProviders(opts, res), newShutdown(nil, nil), nil
	}
	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExporter, traceBatchOptions(opts, cfg.Traces)...),
	)

	metricExporter, err := newMetricExporter(ctx, cfg.Metrics)
	if err != nil {
		_ = tracerProvider.Shutdown(ctx)
		return noopProviders(opts, res), newShutdown(nil, nil), nil
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, metricReaderOptions(opts, cfg.Metrics)...)),
	)

	logExporter, err := newLogExporter(ctx, cfg.Logs)
	if err != nil {
		_ = meterProvider.Shutdown(ctx)
		_ = tracerProvider.Shutdown(ctx)
		return noopProviders(opts, res), newShutdown(nil, nil), nil
	}
	loggerProvider := sdklog.NewLoggerProvider(
		sdklog.WithResource(res),
		sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter, logBatchOptions(opts, cfg.Logs)...)),
	)
	instruments, err := newInstruments(meterProvider.Meter(instrumentationName), opts.Instruments)
	if err != nil {
		_ = loggerProvider.Shutdown(ctx)
		_ = meterProvider.Shutdown(ctx)
		_ = tracerProvider.Shutdown(ctx)
		return nil, nil, err
	}

	providers := &Providers{
		TracerProvider:    tracerProvider,
		MeterProvider:     meterProvider,
		LoggerProvider:    loggerProvider,
		Tracer:            tracerProvider.Tracer(instrumentationName),
		Meter:             meterProvider.Meter(instrumentationName),
		Logger:            loggerProvider.Logger(instrumentationName),
		Instruments:       instruments,
		Resource:          res,
		openLLMetryCompat: opts.OpenLLMetryCompat,
	}

	shutdown := newShutdown(
		[]func(context.Context) error{
			loggerProvider.ForceFlush,
			meterProvider.ForceFlush,
			tracerProvider.ForceFlush,
		},
		[]func(context.Context) error{
			loggerProvider.Shutdown,
			meterProvider.Shutdown,
			tracerProvider.Shutdown,
		},
	)

	return providers, shutdown, nil
}

func newTraceExporter(ctx context.Context, cfg resolvedExporterConfig) (sdktrace.SpanExporter, error) {
	switch cfg.Protocol {
	case ProtocolGRPC:
		return otlptracegrpc.New(ctx, traceGRPCOptions(cfg)...)
	case ProtocolHTTPProtobuf:
		return otlptracehttp.New(ctx, traceHTTPOptions(cfg, signalTraces)...)
	default:
		return nil, errUnsupportedProtocol(cfg.Protocol)
	}
}

func newMetricExporter(ctx context.Context, cfg resolvedExporterConfig) (sdkmetric.Exporter, error) {
	switch cfg.Protocol {
	case ProtocolGRPC:
		return otlpmetricgrpc.New(ctx, metricGRPCOptions(cfg)...)
	case ProtocolHTTPProtobuf:
		return otlpmetrichttp.New(ctx, metricHTTPOptions(cfg, signalMetrics)...)
	default:
		return nil, errUnsupportedProtocol(cfg.Protocol)
	}
}

func newLogExporter(ctx context.Context, cfg resolvedExporterConfig) (sdklog.Exporter, error) {
	switch cfg.Protocol {
	case ProtocolGRPC:
		return otlploggrpc.New(ctx, logGRPCOptions(cfg)...)
	case ProtocolHTTPProtobuf:
		return otlploghttp.New(ctx, logHTTPOptions(cfg, signalLogs)...)
	default:
		return nil, errUnsupportedProtocol(cfg.Protocol)
	}
}

func traceGRPCOptions(cfg resolvedExporterConfig) []otlptracegrpc.Option {
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracegrpc.WithTimeout(cfg.Timeout))
	}
	return opts
}

func metricGRPCOptions(cfg resolvedExporterConfig) []otlpmetricgrpc.Option {
	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlpmetricgrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetricgrpc.WithTimeout(cfg.Timeout))
	}
	return opts
}

func logGRPCOptions(cfg resolvedExporterConfig) []otlploggrpc.Option {
	opts := []otlploggrpc.Option{otlploggrpc.WithEndpoint(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlploggrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploggrpc.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlploggrpc.WithTimeout(cfg.Timeout))
	}
	return opts
}

func traceHTTPOptions(cfg resolvedExporterConfig, sig signal) []otlptracehttp.Option {
	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.Endpoint)}
	if path := defaultHTTPPath(cfg.Endpoint, sig); path != "" {
		opts = append(opts, otlptracehttp.WithURLPath(path))
	}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlptracehttp.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlptracehttp.WithTimeout(cfg.Timeout))
	}
	return opts
}

func metricHTTPOptions(cfg resolvedExporterConfig, sig signal) []otlpmetrichttp.Option {
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpointURL(cfg.Endpoint)}
	if path := defaultHTTPPath(cfg.Endpoint, sig); path != "" {
		opts = append(opts, otlpmetrichttp.WithURLPath(path))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlpmetrichttp.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlpmetrichttp.WithTimeout(cfg.Timeout))
	}
	return opts
}

func logHTTPOptions(cfg resolvedExporterConfig, sig signal) []otlploghttp.Option {
	opts := []otlploghttp.Option{otlploghttp.WithEndpointURL(cfg.Endpoint)}
	if path := defaultHTTPPath(cfg.Endpoint, sig); path != "" {
		opts = append(opts, otlploghttp.WithURLPath(path))
	}
	if cfg.Insecure {
		opts = append(opts, otlploghttp.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(cfg.Headers))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, otlploghttp.WithTimeout(cfg.Timeout))
	}
	return opts
}

func traceBatchOptions(opts Options, cfg resolvedExporterConfig) []sdktrace.BatchSpanProcessorOption {
	var out []sdktrace.BatchSpanProcessorOption
	if opts.BatchTimeout > 0 {
		out = append(out, sdktrace.WithBatchTimeout(opts.BatchTimeout))
	}
	if cfg.Timeout > 0 {
		out = append(out, sdktrace.WithExportTimeout(cfg.Timeout))
	}
	return out
}

func metricReaderOptions(opts Options, cfg resolvedExporterConfig) []sdkmetric.PeriodicReaderOption {
	var out []sdkmetric.PeriodicReaderOption
	if opts.BatchTimeout > 0 {
		out = append(out, sdkmetric.WithInterval(opts.BatchTimeout))
	}
	if cfg.Timeout > 0 {
		out = append(out, sdkmetric.WithTimeout(cfg.Timeout))
	}
	return out
}

func logBatchOptions(opts Options, cfg resolvedExporterConfig) []sdklog.BatchProcessorOption {
	var out []sdklog.BatchProcessorOption
	if opts.BatchTimeout > 0 {
		out = append(out, sdklog.WithExportInterval(opts.BatchTimeout))
	}
	if cfg.Timeout > 0 {
		out = append(out, sdklog.WithExportTimeout(cfg.Timeout))
	}
	return out
}

func exporterEndpointsReachable(ctx context.Context, cfg resolvedConfig, timeout time.Duration) bool {
	for _, exporter := range []resolvedExporterConfig{cfg.Traces, cfg.Metrics, cfg.Logs} {
		if err := dialEndpoint(ctx, exporter, timeout); err != nil {
			return false
		}
	}
	return true
}

func dialEndpoint(ctx context.Context, cfg resolvedExporterConfig, timeout time.Duration) error {
	target := cfg.Endpoint
	if cfg.Protocol == ProtocolHTTPProtobuf {
		parsed, err := url.Parse(cfg.Endpoint)
		if err != nil {
			return err
		}
		target = parsed.Host
	}
	if timeout <= 0 {
		timeout = defaultDialTimeout
	}
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return err
	}
	return conn.Close()
}

func defaultHTTPPath(endpoint string, sig signal) string {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Path != "" && u.Path != "/") {
		return ""
	}
	switch sig {
	case signalTraces:
		return "/v1/traces"
	case signalMetrics:
		return "/v1/metrics"
	case signalLogs:
		return "/v1/logs"
	default:
		return ""
	}
}

func errUnsupportedProtocol(protocol string) error {
	return errors.New("unsupported OTLP protocol " + protocol)
}
