package agentotel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mattsp1290/agent-otel/internal/otlptest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func TestInstrumentsRecordModelMetrics(t *testing.T) {
	providers, shutdown, receiver := startMetricTestProviders(t, InstrumentOptions{})
	labels := ModelMetricLabels{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
	}

	require.NoError(t, providers.Instruments.RecordModelLatency(t.Context(), 0.25, labels))
	require.NoError(t, providers.Instruments.RecordUsage(t.Context(), Usage{
		InputTokens:  100,
		OutputTokens: 180,
		Available:    true,
	}, labels))
	require.NoError(t, providers.Instruments.RecordProviderError(t.Context(), ProviderErrorLabels{
		ProviderName: "openai",
		ErrorType:    "provider_error",
	}))
	require.NoError(t, providers.Instruments.RecordFallbackEngaged(t.Context(), Fallback{
		ProviderName: "openai",
		FromProvider: "openai",
		ToProvider:   "anthropic",
	}))
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	snap := waitForMetrics(t, receiver, MetricGenAIClientOperationDuration, MetricGenAIClientTokenUsage, MetricAgentOTelProviderErrors, MetricAgentOTelFallbackEngaged)

	latency := findMetric(snap, MetricGenAIClientOperationDuration)
	require.Equal(t, "s", latency.GetUnit())
	latencyPoint := latency.GetHistogram().GetDataPoints()[0]
	require.Equal(t, uint64(1), latencyPoint.GetCount())
	require.Equal(t, 0.25, latencyPoint.GetSum())
	assertMetricAttrs(t, latencyPoint.GetAttributes(), map[string]string{
		AttrGenAIOperationName: GenAIOperationChat,
		AttrGenAIProviderName:  "openai",
		AttrGenAIRequestModel:  "gpt-4o",
	})

	tokenMetric := findMetric(snap, MetricGenAIClientTokenUsage)
	require.Equal(t, "{token}", tokenMetric.GetUnit())
	tokenPoints := tokenMetric.GetHistogram().GetDataPoints()
	require.Len(t, tokenPoints, 2)
	assertHistogramPoint(t, tokenPoints, GenAITokenTypeInput, 100)
	assertHistogramPoint(t, tokenPoints, GenAITokenTypeOutput, 180)

	errorsMetric := findMetric(snap, MetricAgentOTelProviderErrors)
	require.Equal(t, "{error}", errorsMetric.GetUnit())
	errorPoint := errorsMetric.GetSum().GetDataPoints()[0]
	require.Equal(t, int64(1), errorPoint.GetAsInt())
	assertMetricAttrs(t, errorPoint.GetAttributes(), map[string]string{
		AttrGenAIProviderName: "openai",
		AttrErrorType:         "provider_error",
	})

	fallbackMetric := findMetric(snap, MetricAgentOTelFallbackEngaged)
	require.Equal(t, "{event}", fallbackMetric.GetUnit())
	fallbackPoint := fallbackMetric.GetSum().GetDataPoints()[0]
	require.Equal(t, int64(1), fallbackPoint.GetAsInt())
	assertMetricAttrs(t, fallbackPoint.GetAttributes(), map[string]string{
		AttrGenAIProviderName:     "openai",
		AttrAgentOTelProviderFrom: "openai",
		AttrAgentOTelProviderTo:   "anthropic",
	})
}

func TestInstrumentsSkipUnavailableUsage(t *testing.T) {
	providers, shutdown, receiver := startMetricTestProviders(t, InstrumentOptions{})
	labels := ModelMetricLabels{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
	}

	require.NoError(t, providers.Instruments.RecordUsage(t.Context(), Usage{
		InputTokens:  100,
		OutputTokens: 180,
		Available:    false,
	}, labels))
	require.NoError(t, providers.Instruments.RecordModelLatency(t.Context(), 0.5, labels))
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	snap := waitForMetrics(t, receiver, MetricGenAIClientOperationDuration)
	require.NotNil(t, findMetric(snap, MetricGenAIClientOperationDuration))
	require.Nil(t, findMetric(snap, MetricGenAIClientTokenUsage))
}

func TestInstrumentsFilterProhibitedCardinalityLabels(t *testing.T) {
	providers, shutdown, receiver := startMetricTestProviders(t, InstrumentOptions{})

	require.NoError(t, providers.Instruments.RecordFallbackEngaged(t.Context(), Fallback{
		ProviderName: "openai",
		FromProvider: "openai",
		ToProvider:   "anthropic",
		Attributes: []attribute.KeyValue{
			attribute.String("session.id", "high-cardinality"),
		},
	}))
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	snap := waitForMetrics(t, receiver, MetricAgentOTelFallbackEngaged)
	point := findMetric(snap, MetricAgentOTelFallbackEngaged).GetSum().GetDataPoints()[0]
	attrs := kvAttrs(point.GetAttributes())
	require.NotContains(t, attrs, "session.id")
	require.Equal(t, "openai", attrs[AttrAgentOTelProviderFrom].GetStringValue())
}

func TestInstrumentsStrictCardinalityReturnsError(t *testing.T) {
	providers, _, _ := startMetricTestProviders(t, InstrumentOptions{CardinalityMode: CardinalityStrict})

	err := providers.Instruments.RecordFallbackEngaged(t.Context(), Fallback{
		ProviderName: "openai",
		FromProvider: "openai",
		ToProvider:   "anthropic",
		Attributes: []attribute.KeyValue{
			attribute.String("session.id", "high-cardinality"),
		},
	})
	require.Error(t, err)
}

func TestInstrumentsRejectNegativeUsage(t *testing.T) {
	providers, _, _ := startMetricTestProviders(t, InstrumentOptions{})

	err := providers.Instruments.RecordUsage(t.Context(), Usage{InputTokens: -1, Available: true}, ModelMetricLabels{})
	require.ErrorIs(t, err, ErrNegativeUsage)
	err = providers.Instruments.RecordUsageInputTokens(t.Context(), -1, ModelMetricLabels{})
	require.True(t, errors.Is(err, ErrNegativeUsage))
}

func startMetricTestProviders(t *testing.T, instrumentOptions InstrumentOptions) (*Providers, *Shutdown, *otlptest.Receiver) {
	t.Helper()
	clearOTLPEnv(t)
	receiver := otlptest.Start(t)
	providers, shutdown, err := Init(t.Context(), Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		Instruments:       instrumentOptions,
		TraceExporter: ExporterConfig{
			Endpoint: receiver.Endpoint(),
			Protocol: ProtocolGRPC,
			Insecure: true,
		},
		MetricExporter: ExporterConfig{
			Endpoint: receiver.Endpoint(),
			Protocol: ProtocolGRPC,
			Insecure: true,
		},
		LogExporter: ExporterConfig{
			Endpoint: receiver.Endpoint(),
			Protocol: ProtocolGRPC,
			Insecure: true,
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = shutdown.Shutdown(context.Background()) })
	return providers, shutdown, receiver
}

func waitForMetrics(t *testing.T, receiver *otlptest.Receiver, names ...string) otlptest.Snapshot {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	snap, err := receiver.WaitFor(ctx, func(s otlptest.Snapshot) bool {
		for _, name := range names {
			if findMetric(s, name) == nil {
				return false
			}
		}
		return true
	})
	require.NoError(t, err)
	return snap
}

func findMetric(snap otlptest.Snapshot, name string) *metricsv1.Metric {
	for _, req := range snap.Metrics {
		for _, resourceMetrics := range req.GetResourceMetrics() {
			for _, scopeMetrics := range resourceMetrics.GetScopeMetrics() {
				for _, metric := range scopeMetrics.GetMetrics() {
					if metric.GetName() == name {
						return metric
					}
				}
			}
		}
	}
	return nil
}

func assertHistogramPoint(t *testing.T, points []*metricsv1.HistogramDataPoint, tokenType string, wantSum float64) {
	t.Helper()
	for _, point := range points {
		attrs := kvAttrs(point.GetAttributes())
		if attrs[AttrGenAITokenType].GetStringValue() != tokenType {
			continue
		}
		require.Equal(t, uint64(1), point.GetCount())
		require.Equal(t, wantSum, point.GetSum())
		assertMetricAttrs(t, point.GetAttributes(), map[string]string{
			AttrGenAIOperationName: GenAIOperationChat,
			AttrGenAIProviderName:  "openai",
			AttrGenAIRequestModel:  "gpt-4o",
			AttrGenAITokenType:     tokenType,
		})
		return
	}
	require.FailNow(t, "token histogram point not found", tokenType)
}

func assertMetricAttrs(t *testing.T, attrs []*commonv1.KeyValue, want map[string]string) {
	t.Helper()
	got := kvAttrs(attrs)
	for key, value := range want {
		require.Contains(t, got, key)
		require.Equal(t, value, got[key].GetStringValue())
	}
}
