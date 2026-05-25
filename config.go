package agentotel

import (
	"fmt"
	"time"
)

const (
	// ProtocolGRPC selects OTLP/gRPC export.
	ProtocolGRPC = "grpc"
	// ProtocolHTTPProtobuf selects OTLP/HTTP protobuf export.
	ProtocolHTTPProtobuf = "http/protobuf"

	defaultGRPCEndpoint = "localhost:4317"
	defaultHTTPEndpoint = "http://localhost:4318"
)

type signal string

const (
	signalTraces  signal = "traces"
	signalMetrics signal = "metrics"
	signalLogs    signal = "logs"
)

// ConfigValidationError reports malformed telemetry configuration.
type ConfigValidationError struct {
	Field  string
	Value  string
	Reason string
	Err    error
}

func (e *ConfigValidationError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("invalid %s=%q: %s", e.Field, e.Value, e.Reason)
}

// Unwrap returns the underlying parse or validation error.
func (e *ConfigValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

type resolvedConfig struct {
	Traces  resolvedExporterConfig
	Metrics resolvedExporterConfig
	Logs    resolvedExporterConfig
}

type resolvedExporterConfig struct {
	Endpoint string
	Protocol string
	Headers  map[string]string
	Insecure bool
	Timeout  time.Duration
}

func resolveConfig(opts Options, lookup lookupEnv) (resolvedConfig, error) {
	traces, err := resolveExporterConfig(signalTraces, opts.TraceExporter, opts, lookup)
	if err != nil {
		return resolvedConfig{}, err
	}
	metrics, err := resolveExporterConfig(signalMetrics, opts.MetricExporter, opts, lookup)
	if err != nil {
		return resolvedConfig{}, err
	}
	logs, err := resolveExporterConfig(signalLogs, opts.LogExporter, opts, lookup)
	if err != nil {
		return resolvedConfig{}, err
	}
	return resolvedConfig{
		Traces:  traces,
		Metrics: metrics,
		Logs:    logs,
	}, nil
}

func resolveExporterConfig(sig signal, explicit ExporterConfig, opts Options, lookup lookupEnv) (resolvedExporterConfig, error) {
	protocol, protocolField, protocolValue := resolveProtocolWithPreset(sig, explicit, opts.DatadogPreset, lookup)
	if err := validateProtocol(protocolField, protocolValue, protocol); err != nil {
		return resolvedExporterConfig{}, err
	}

	endpoint, endpointField, endpointValue := resolveEndpointWithPreset(sig, explicit, protocol, opts.DatadogPreset, lookup)
	insecure := resolveInsecure(sig, explicit, endpoint, lookup)
	normalizedEndpoint, normalizedInsecure, err := normalizeEndpointForProtocol(endpointField, endpointValue, endpoint, protocol, insecure)
	if err != nil {
		return resolvedExporterConfig{}, err
	}

	headers := resolveHeadersWithPreset(sig, explicit, opts.DatadogPreset, lookup)
	timeout, err := resolveTimeout(sig, explicit, opts, lookup)
	if err != nil {
		return resolvedExporterConfig{}, err
	}

	return resolvedExporterConfig{
		Endpoint: normalizedEndpoint,
		Protocol: protocol,
		Headers:  headers,
		Insecure: normalizedInsecure,
		Timeout:  timeout,
	}, nil
}
