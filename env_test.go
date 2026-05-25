package agentotel

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type envMap map[string]string

func (m envMap) lookup(name string) (string, bool) {
	value, ok := m[name]
	return value, ok
}

func TestResolveConfigPerSignalEndpointPrecedence(t *testing.T) {
	cfg, err := resolveConfig(Options{}, envMap{
		EnvOTLPEndpoint:                       "general:4317",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  "traces:4317",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "metrics:4317",
		"OTEL_EXPORTER_OTLP_LOGS_ENDPOINT":    "logs:4317",
		"OTEL_EXPORTER_OTLP_TRACES_INSECURE":  "true",
		"OTEL_EXPORTER_OTLP_METRICS_INSECURE": "true",
		"OTEL_EXPORTER_OTLP_LOGS_INSECURE":    "true",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, "traces:4317", cfg.Traces.Endpoint)
	require.Equal(t, "metrics:4317", cfg.Metrics.Endpoint)
	require.Equal(t, "logs:4317", cfg.Logs.Endpoint)
	require.True(t, cfg.Traces.Insecure)
	require.True(t, cfg.Metrics.Insecure)
	require.True(t, cfg.Logs.Insecure)
}

func TestResolveConfigGeneralFallback(t *testing.T) {
	cfg, err := resolveConfig(Options{}, envMap{
		EnvOTLPEndpoint: "general:4317",
		EnvOTLPInsecure: "true",
		EnvOTLPProtocol: ProtocolGRPC,
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, "general:4317", cfg.Traces.Endpoint)
	require.Equal(t, "general:4317", cfg.Metrics.Endpoint)
	require.Equal(t, "general:4317", cfg.Logs.Endpoint)
	require.True(t, cfg.Traces.Insecure)
	require.True(t, cfg.Metrics.Insecure)
	require.True(t, cfg.Logs.Insecure)
}

func TestResolveConfigExplicitOptionsBeatEnv(t *testing.T) {
	cfg, err := resolveConfig(Options{
		TraceExporter: ExporterConfig{
			Endpoint: "explicit:4317",
			Protocol: ProtocolGRPC,
			Headers: map[string]string{
				"x-explicit": "yes",
			},
			Insecure: true,
			Timeout:  9 * time.Second,
		},
	}, envMap{
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "env:4317",
		"OTEL_EXPORTER_OTLP_TRACES_HEADERS":  "x-env=no",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": ProtocolHTTPProtobuf,
		"OTEL_EXPORTER_OTLP_TRACES_TIMEOUT":  "100",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, "explicit:4317", cfg.Traces.Endpoint)
	require.Equal(t, ProtocolGRPC, cfg.Traces.Protocol)
	require.True(t, cfg.Traces.Insecure)
	require.Equal(t, map[string]string{"x-explicit": "yes"}, cfg.Traces.Headers)
	require.Equal(t, 9*time.Second, cfg.Traces.Timeout)
}

func TestResolveConfigHeaders(t *testing.T) {
	cfg, err := resolveConfig(Options{}, envMap{
		EnvOTLPHeaders:                      "authorization=Bearer%20general,x-team=agent",
		"OTEL_EXPORTER_OTLP_TRACES_HEADERS": "authorization=Bearer%20trace%3Donly",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, map[string]string{"authorization": "Bearer trace=only"}, cfg.Traces.Headers)
	require.Equal(t, map[string]string{"authorization": "Bearer general", "x-team": "agent"}, cfg.Metrics.Headers)
	require.Equal(t, map[string]string{"authorization": "Bearer general", "x-team": "agent"}, cfg.Logs.Headers)
}

func TestResolveConfigProtocolPrecedence(t *testing.T) {
	cfg, err := resolveConfig(Options{}, envMap{
		EnvOTLPProtocol:                      ProtocolGRPC,
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": ProtocolHTTPProtobuf,
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, ProtocolHTTPProtobuf, cfg.Traces.Protocol)
	require.Equal(t, ProtocolGRPC, cfg.Metrics.Protocol)
	require.Equal(t, ProtocolGRPC, cfg.Logs.Protocol)
	require.Equal(t, defaultHTTPEndpoint, cfg.Traces.Endpoint)
	require.Equal(t, defaultGRPCEndpoint, cfg.Metrics.Endpoint)
}

func TestResolveConfigGRPCSchemeDerivedTLS(t *testing.T) {
	tests := []struct {
		name         string
		endpoint     string
		insecureEnv  string
		wantEndpoint string
		wantInsecure bool
	}{
		{
			name:         "http scheme forces plaintext",
			endpoint:     "http://collector:4317",
			insecureEnv:  "false",
			wantEndpoint: "collector:4317",
			wantInsecure: true,
		},
		{
			name:         "https scheme forces tls",
			endpoint:     "https://collector:4317",
			insecureEnv:  "true",
			wantEndpoint: "collector:4317",
			wantInsecure: false,
		},
		{
			name:         "bare endpoint uses insecure env",
			endpoint:     "collector:4317",
			insecureEnv:  "true",
			wantEndpoint: "collector:4317",
			wantInsecure: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := resolveConfig(Options{}, envMap{
				EnvOTLPEndpoint: tt.endpoint,
				EnvOTLPInsecure: tt.insecureEnv,
			}.lookup)
			require.NoError(t, err)
			require.Equal(t, tt.wantEndpoint, cfg.Traces.Endpoint)
			require.Equal(t, tt.wantInsecure, cfg.Traces.Insecure)
		})
	}
}

func TestResolveConfigHTTPURLNormalization(t *testing.T) {
	cfg, err := resolveConfig(Options{}, envMap{
		EnvOTLPProtocol:                       ProtocolHTTPProtobuf,
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT":  "https://collector.example:4318/v1/traces",
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT": "collector.example:4318",
		"OTEL_EXPORTER_OTLP_METRICS_INSECURE": "true",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, "https://collector.example:4318/v1/traces", cfg.Traces.Endpoint)
	require.False(t, cfg.Traces.Insecure)
	require.Equal(t, "http://collector.example:4318", cfg.Metrics.Endpoint)
	require.True(t, cfg.Metrics.Insecure)
	require.Equal(t, defaultHTTPEndpoint, cfg.Logs.Endpoint)
	require.True(t, cfg.Logs.Insecure)
}

func TestResolveConfigTimeouts(t *testing.T) {
	cfg, err := resolveConfig(Options{ExportTimeout: 3 * time.Second}, envMap{
		EnvOTLPTimeout:                      "2500",
		"OTEL_EXPORTER_OTLP_TRACES_TIMEOUT": "100",
	}.lookup)
	require.NoError(t, err)
	require.Equal(t, 100*time.Millisecond, cfg.Traces.Timeout)
	require.Equal(t, 2500*time.Millisecond, cfg.Metrics.Timeout)
	require.Equal(t, 2500*time.Millisecond, cfg.Logs.Timeout)
}

func TestResolveConfigMalformedConfigurationErrors(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		env     envMap
		wantErr error
	}{
		{
			name: "unsupported protocol",
			env:  envMap{"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": "http/json"},
		},
		{
			name: "grpc whitespace",
			env:  envMap{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "host name:4317"},
		},
		{
			name:    "grpc invalid port",
			env:     envMap{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "collector:abc"},
			wantErr: strconv.ErrSyntax,
		},
		{
			name: "grpc empty host",
			env:  envMap{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": ":4317"},
		},
		{
			name: "http unsupported scheme",
			env: envMap{
				"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": ProtocolHTTPProtobuf,
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "ftp://collector:4318",
			},
		},
		{
			name: "http empty host",
			env: envMap{
				"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL": ProtocolHTTPProtobuf,
				"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "https:///v1/traces",
			},
		},
		{
			name: "option endpoint validates too",
			opts: Options{TraceExporter: ExporterConfig{Endpoint: "collector:70000"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveConfig(tt.opts, tt.env.lookup)
			var validationErr *ConfigValidationError
			require.ErrorAs(t, err, &validationErr)
			if tt.wantErr != nil {
				require.True(t, errors.Is(err, tt.wantErr), "error %v should wrap %v", err, tt.wantErr)
			}
		})
	}
}

func TestInitReturnsMalformedConfigError(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "http/json")

	providers, shutdown, err := Init(t.Context(), Options{
		Enabled:           true,
		SkipGlobalInstall: true,
	})
	require.Nil(t, providers)
	require.Nil(t, shutdown)
	var validationErr *ConfigValidationError
	require.ErrorAs(t, err, &validationErr)
}
