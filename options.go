package agentotel

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Options controls agent OpenTelemetry bootstrap.
type Options struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	Environment    string
	ResourceAttrs  []attribute.KeyValue

	SkipGlobalInstall bool
	DialTimeout       time.Duration
	BatchTimeout      time.Duration
	ExportTimeout     time.Duration

	TraceExporter  ExporterConfig
	MetricExporter ExporterConfig
	LogExporter    ExporterConfig

	DatadogPreset *DatadogPreset
	DevSink       *DevSinkConfig
	Instruments   InstrumentOptions
}

// ExporterConfig describes an OTLP exporter endpoint.
type ExporterConfig struct {
	Endpoint string
	Protocol string
	Headers  map[string]string
	Insecure bool
	Timeout  time.Duration
}

// DatadogPreset holds Datadog-specific bootstrap settings.
type DatadogPreset struct {
	Enabled bool
}

// DevSinkConfig holds local development telemetry sink settings.
type DevSinkConfig struct {
	Enabled bool
}

// InstrumentOptions controls helper instrument construction.
type InstrumentOptions struct {
	CardinalityMode CardinalityMode
}
