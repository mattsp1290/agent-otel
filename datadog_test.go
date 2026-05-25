package agentotel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDatadogPresetTraceDefaults(t *testing.T) {
	opts := ApplyOptions(Options{}, WithDatadogPreset())
	cfg, err := resolveConfig(opts, envMap{
		EnvDatadogAPIKey: "test-api-key",
	}.lookup)
	require.NoError(t, err)

	require.Equal(t, ProtocolHTTPProtobuf, cfg.Traces.Protocol)
	require.Equal(t, defaultHTTPEndpoint, cfg.Traces.Endpoint)
	require.Equal(t, "test-api-key", cfg.Traces.Headers[DatadogHeaderAPIKey])
	require.Equal(t, DatadogOTLPSourceLLMObs, cfg.Traces.Headers[DatadogHeaderOTLPSource])
	require.Equal(t, ProtocolGRPC, cfg.Metrics.Protocol)
	require.Equal(t, ProtocolGRPC, cfg.Logs.Protocol)
	require.Nil(t, cfg.Metrics.Headers)
	require.Nil(t, cfg.Logs.Headers)
	require.Equal(t, DatadogSemconvStabilityOptIn, opts.DatadogPreset.SemconvStabilityOptIn)
	require.False(t, opts.DatadogPreset.PromptPayloadCapture)
	require.False(t, opts.DatadogPreset.OpenLLMetryCompat)
}

func TestDatadogPresetEndpointOverride(t *testing.T) {
	opts := ApplyOptions(Options{}, WithDatadogPreset(
		WithDatadogTraceEndpoint("https://otlp.datadoghq.com/v1/traces"),
		WithDatadogAPIKey("explicit-key"),
		WithDatadogHeaders(map[string]string{"x-extra": "yes"}),
	))
	cfg, err := resolveConfig(opts, envMap{}.lookup)
	require.NoError(t, err)

	require.Equal(t, ProtocolHTTPProtobuf, cfg.Traces.Protocol)
	require.Equal(t, "https://otlp.datadoghq.com/v1/traces", cfg.Traces.Endpoint)
	require.False(t, cfg.Traces.Insecure)
	require.Equal(t, map[string]string{
		DatadogHeaderAPIKey:     "explicit-key",
		DatadogHeaderOTLPSource: DatadogOTLPSourceLLMObs,
		"x-extra":               "yes",
	}, cfg.Traces.Headers)
}

func TestDatadogPresetRespectsExplicitAndEnvOverrides(t *testing.T) {
	opts := ApplyOptions(Options{
		TraceExporter: ExporterConfig{
			Endpoint: "explicit:4317",
			Protocol: ProtocolGRPC,
			Headers:  map[string]string{"authorization": "explicit"},
			Insecure: true,
		},
	}, WithDatadogPreset(WithDatadogTraceEndpoint("https://otlp.datadoghq.com/v1/traces")))
	cfg, err := resolveConfig(opts, envMap{
		EnvDatadogAPIKey:                     "test-api-key",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "env:4317",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": ProtocolHTTPProtobuf,
		"OTEL_EXPORTER_OTLP_TRACES_HEADERS":  "authorization=env",
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE": "false",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, "explicit:4317", cfg.Traces.Endpoint)
	require.Equal(t, ProtocolGRPC, cfg.Traces.Protocol)
	require.Equal(t, map[string]string{"authorization": "explicit"}, cfg.Traces.Headers)
	require.True(t, cfg.Traces.Insecure)

	envOpts := ApplyOptions(Options{}, WithDatadogPreset(WithDatadogTraceEndpoint("https://otlp.datadoghq.com/v1/traces")))
	cfg, err = resolveConfig(envOpts, envMap{
		EnvDatadogAPIKey:                     "test-api-key",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "env:4317",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": ProtocolGRPC,
		"OTEL_EXPORTER_OTLP_TRACES_HEADERS":  "authorization=env",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, "env:4317", cfg.Traces.Endpoint)
	require.Equal(t, ProtocolGRPC, cfg.Traces.Protocol)
	require.Equal(t, map[string]string{"authorization": "env"}, cfg.Traces.Headers)
}

func TestDatadogPresetIsOptIn(t *testing.T) {
	cfg, err := resolveConfig(Options{}, envMap{
		EnvDatadogAPIKey: "test-api-key",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, ProtocolGRPC, cfg.Traces.Protocol)
	require.Equal(t, defaultGRPCEndpoint, cfg.Traces.Endpoint)
	require.Nil(t, cfg.Traces.Headers)
}

func TestDatadogPresetOnWireHeadersAndGenAIAttrs(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv(EnvDatadogAPIKey, "test-api-key")
	receiver := startHTTPOTLPReceiver(t)

	opts := ApplyOptions(Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		TraceExporter: ExporterConfig{
			Endpoint: receiver.URL(),
		},
		MetricExporter: ExporterConfig{
			Endpoint: receiver.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		LogExporter: ExporterConfig{
			Endpoint: receiver.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
	}, WithDatadogPreset())
	cfg, err := resolveConfig(opts, processEnv)
	require.NoError(t, err)
	require.Equal(t, "test-api-key", cfg.Traces.Headers[DatadogHeaderAPIKey])

	providers, shutdown, err := Init(t.Context(), opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = shutdown.Shutdown(context.Background()) })
	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		ResponseModel: "gpt-4o-2024-08-06",
		Usage:         Usage{InputTokens: 100, OutputTokens: 180, Available: true},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	snap := receiver.WaitFor(t, func(s httpOTLPSnapshot) bool {
		return len(s.Traces) > 0
	})
	require.NotEmpty(t, receiver.TraceHeaders())
	headers := receiver.TraceHeaders()[0]
	require.Equal(t, "test-api-key", headers.Get(DatadogHeaderAPIKey), "headers=%v", headers)
	require.Equal(t, DatadogOTLPSourceLLMObs, headers.Get(DatadogHeaderOTLPSource))

	wireSpan := snap.Traces[0].GetResourceSpans()[0].GetScopeSpans()[0].GetSpans()[0]
	attrs := spanAttrs(wireSpan)
	require.Equal(t, GenAIOperationChat, attrs[AttrGenAIOperationName].GetStringValue())
	require.Equal(t, "openai", attrs[AttrGenAIProviderName].GetStringValue())
	require.Equal(t, "gpt-4o", attrs[AttrGenAIRequestModel].GetStringValue())
	require.Equal(t, "gpt-4o-2024-08-06", attrs[AttrGenAIResponseModel].GetStringValue())
	require.Equal(t, int64(100), attrs[AttrGenAIUsageInputTokens].GetIntValue())
	require.Equal(t, int64(180), attrs[AttrGenAIUsageOutputTokens].GetIntValue())
	require.NotContains(t, attrs, AttrGenAIInputMessages)
	require.NotContains(t, attrs, AttrGenAIOutputMessages)
}
