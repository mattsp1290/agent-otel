package agentotel

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

func TestInitDisabledReturnsNoopProviders(t *testing.T) {
	clearOTLPEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "http/json")

	providers, shutdown, err := Init(context.Background(), Options{
		Enabled:           false,
		SkipGlobalInstall: true,
	})
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NotNil(t, providers.TracerProvider)
	require.NotNil(t, providers.MeterProvider)
	require.NotNil(t, providers.LoggerProvider)
	require.NotNil(t, providers.Tracer)
	require.NotNil(t, providers.Meter)
	require.NotNil(t, providers.Logger)
	require.NotNil(t, providers.Instruments)
	require.NotNil(t, providers.Resource)
	require.NotNil(t, shutdown)
	require.NoError(t, shutdown.ForceFlush(context.Background()))
	require.NoError(t, shutdown.Shutdown(context.Background()))
	require.NoError(t, shutdown.Shutdown(context.Background()))
}

func TestInitEnabledWithoutCollectorDoesNotPanic(t *testing.T) {
	clearOTLPEnv(t)
	providers, shutdown, err := Init(context.Background(), Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		TraceExporter: ExporterConfig{
			Endpoint: "127.0.0.1:1",
			Protocol: "grpc",
			Insecure: true,
		},
		MetricExporter: ExporterConfig{
			Endpoint: "127.0.0.1:1",
			Protocol: "grpc",
			Insecure: true,
		},
		LogExporter: ExporterConfig{
			Endpoint: "127.0.0.1:1",
			Protocol: "grpc",
			Insecure: true,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NotNil(t, shutdown)
	require.NoError(t, shutdown.ForceFlush(context.Background()))
	require.NoError(t, shutdown.Shutdown(context.Background()))
}

func TestInitBuildsResourceAttributes(t *testing.T) {
	clearOTLPEnv(t)
	providers, _, err := Init(context.Background(), Options{
		Enabled:           false,
		SkipGlobalInstall: true,
		ServiceName:       "advisor",
		ServiceVersion:    "1.2.3",
		Environment:       "test",
		ResourceAttrs: []attribute.KeyValue{
			attribute.String("custom.attr", "custom-value"),
		},
	})
	require.NoError(t, err)

	attrs := providers.Resource.Set()
	assertResourceString(t, attrs, semconv.ServiceNameKey, "advisor")
	assertResourceString(t, attrs, semconv.ServiceVersionKey, "1.2.3")
	assertResourceString(t, attrs, semconv.DeploymentEnvironmentNameKey, "test")
	assertResourceString(t, attrs, attribute.Key("custom.attr"), "custom-value")
}

func TestShutdownOrderAndIdempotence(t *testing.T) {
	forceErr := errors.New("force")
	shutdownErr := errors.New("shutdown")
	var calls []string
	cleanup := newShutdown(
		[]func(context.Context) error{
			func(context.Context) error {
				calls = append(calls, "force logs")
				return nil
			},
			func(context.Context) error {
				calls = append(calls, "force metrics")
				return forceErr
			},
			func(context.Context) error {
				calls = append(calls, "force traces")
				return nil
			},
		},
		[]func(context.Context) error{
			func(context.Context) error {
				calls = append(calls, "shutdown logs")
				return nil
			},
			func(context.Context) error {
				calls = append(calls, "shutdown metrics")
				return shutdownErr
			},
			func(context.Context) error {
				calls = append(calls, "shutdown traces")
				return nil
			},
		},
	)

	err := cleanup.ForceFlush(context.Background())
	require.ErrorIs(t, err, forceErr)
	err = cleanup.Shutdown(context.Background())
	require.ErrorIs(t, err, shutdownErr)
	err = cleanup.Shutdown(context.Background())
	require.ErrorIs(t, err, shutdownErr)
	require.Equal(t, []string{
		"force logs",
		"force metrics",
		"force traces",
		"shutdown logs",
		"shutdown metrics",
		"shutdown traces",
	}, calls)
}

func assertResourceString(t *testing.T, attrs interface {
	Value(attribute.Key) (attribute.Value, bool)
}, key attribute.Key, want string) {
	t.Helper()
	value, ok := attrs.Value(key)
	require.True(t, ok)
	require.Equal(t, want, value.AsString())
}
