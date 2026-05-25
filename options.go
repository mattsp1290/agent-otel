package agentotel

import (
	"maps"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// Option mutates Options for callers that prefer preset-style construction.
type Option func(*Options)

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

	OpenLLMetryCompat *OpenLLMetryCompatOptions
	PayloadCapture    *PayloadCaptureOptions
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
	Enabled               bool
	TraceEndpoint         string
	APIKey                string
	Headers               map[string]string
	SemconvStabilityOptIn string
	PromptPayloadCapture  bool
	OpenLLMetryCompat     bool
}

// DatadogPresetOption mutates Datadog preset settings.
type DatadogPresetOption func(*DatadogPreset)

// DevSinkConfig holds local development telemetry sink settings.
type DevSinkConfig struct {
	Enabled  bool
	Endpoint string
}

// InstrumentOptions controls helper instrument construction.
type InstrumentOptions struct {
	CardinalityMode CardinalityMode
}

// ApplyOptions applies functional options to base and returns the result.
func ApplyOptions(base Options, options ...Option) Options {
	for _, opt := range options {
		if opt != nil {
			opt(&base)
		}
	}
	return base
}

// WithDatadogPreset enables Datadog LLM Observability exporter defaults.
func WithDatadogPreset(options ...DatadogPresetOption) Option {
	return func(opts *Options) {
		preset := defaultDatadogPreset()
		if opts.DatadogPreset != nil {
			preset = *opts.DatadogPreset
			preset.Enabled = true
			preset.Headers = cloneStringMap(preset.Headers)
		}
		for _, opt := range options {
			if opt != nil {
				opt(&preset)
			}
		}
		opts.DatadogPreset = &preset
	}
}

// WithDatadogTraceEndpoint sets the Datadog OTLP traces endpoint.
func WithDatadogTraceEndpoint(endpoint string) DatadogPresetOption {
	return func(preset *DatadogPreset) {
		preset.TraceEndpoint = endpoint
	}
}

// WithDatadogAPIKey sets the Datadog API key header value.
func WithDatadogAPIKey(apiKey string) DatadogPresetOption {
	return func(preset *DatadogPreset) {
		preset.APIKey = apiKey
	}
}

// WithDatadogHeaders adds Datadog trace headers without overriding explicit headers.
func WithDatadogHeaders(headers map[string]string) DatadogPresetOption {
	return func(preset *DatadogPreset) {
		if len(headers) == 0 {
			return
		}
		if preset.Headers == nil {
			preset.Headers = make(map[string]string, len(headers))
		}
		for key, value := range headers {
			preset.Headers[key] = value
		}
	}
}

// WithDevSink duplicates telemetry to a local lotel OTLP endpoint.
func WithDevSink(lotelEndpoint string) Option {
	return func(opts *Options) {
		opts.DevSink = &DevSinkConfig{
			Enabled:  true,
			Endpoint: lotelEndpoint,
		}
	}
}

func defaultDatadogPreset() DatadogPreset {
	return DatadogPreset{
		Enabled:               true,
		SemconvStabilityOptIn: DatadogSemconvStabilityOptIn,
	}
}

func cloneStringMap(in map[string]string) map[string]string {
	return maps.Clone(in)
}
