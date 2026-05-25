package agentotel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestWithDevSinkExportsToPrimaryAndLotel(t *testing.T) {
	clearOTLPEnv(t)
	primary := startHTTPOTLPReceiver(t)
	dev := startHTTPOTLPReceiver(t)

	providers, shutdown, err := Init(t.Context(), ApplyOptions(Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		TraceExporter: ExporterConfig{
			Endpoint: primary.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		MetricExporter: ExporterConfig{
			Endpoint: primary.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		LogExporter: ExporterConfig{
			Endpoint: primary.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
	}, WithDevSink(dev.URL())))
	require.NoError(t, err)

	emitTestTelemetry(t, providers)
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	for _, receiver := range []*httpOTLPReceiver{primary, dev} {
		snap := receiver.WaitFor(t, func(s httpOTLPSnapshot) bool {
			return len(s.Traces) > 0 && len(s.Metrics) > 0 && len(s.Logs) > 0
		})
		require.NotEmpty(t, snap.Traces[0].GetResourceSpans())
		require.NotEmpty(t, snap.Metrics[0].GetResourceMetrics())
		require.NotEmpty(t, snap.Logs[0].GetResourceLogs())
	}
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func TestWithDevSinkMissingLotelDoesNotDisablePrimary(t *testing.T) {
	clearOTLPEnv(t)
	primary := startHTTPOTLPReceiver(t)

	providers, shutdown, err := Init(t.Context(), ApplyOptions(Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		DialTimeout:       10 * time.Millisecond,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		TraceExporter: ExporterConfig{
			Endpoint: primary.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		MetricExporter: ExporterConfig{
			Endpoint: primary.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		LogExporter: ExporterConfig{
			Endpoint: primary.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
	}, WithDevSink("127.0.0.1:1")))
	require.NoError(t, err)

	emitTestTelemetry(t, providers)
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	snap := primary.WaitFor(t, func(s httpOTLPSnapshot) bool {
		return len(s.Traces) > 0 && len(s.Metrics) > 0 && len(s.Logs) > 0
	})
	require.NotEmpty(t, snap.Traces[0].GetResourceSpans())
	require.NotEmpty(t, snap.Metrics[0].GetResourceMetrics())
	require.NotEmpty(t, snap.Logs[0].GetResourceLogs())
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func TestWithDevSinkPrimaryRemainsAuthoritative(t *testing.T) {
	clearOTLPEnv(t)
	dev := startHTTPOTLPReceiver(t)

	providers, shutdown, err := Init(t.Context(), ApplyOptions(Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		DialTimeout:       10 * time.Millisecond,
		ExportTimeout:     10 * time.Millisecond,
		TraceExporter: ExporterConfig{
			Endpoint: "127.0.0.1:1",
			Protocol: ProtocolGRPC,
			Insecure: true,
		},
		MetricExporter: ExporterConfig{
			Endpoint: "127.0.0.1:1",
			Protocol: ProtocolGRPC,
			Insecure: true,
		},
		LogExporter: ExporterConfig{
			Endpoint: "127.0.0.1:1",
			Protocol: ProtocolGRPC,
			Insecure: true,
		},
	}, WithDevSink(dev.URL())))
	require.NoError(t, err)

	emitTestTelemetry(t, providers)
	require.NoError(t, shutdown.ForceFlush(t.Context()))
	require.Empty(t, dev.Snapshot().Traces)
	require.Empty(t, dev.Snapshot().Metrics)
	require.Empty(t, dev.Snapshot().Logs)
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func TestDevSinkFanoutOrderAndFailOpen(t *testing.T) {
	primaryErr := errors.New("primary")
	devErr := errors.New("dev")
	var calls []string

	metricExporter := multiMetricExporter{
		primary: &recordingMetricExporter{calls: &calls, name: "primary metric", err: primaryErr},
		dev:     &recordingMetricExporter{calls: &calls, name: "dev metric", err: devErr},
	}
	require.ErrorIs(t, metricExporter.Export(t.Context(), &metricdata.ResourceMetrics{}), primaryErr)
	require.ErrorIs(t, metricExporter.ForceFlush(t.Context()), primaryErr)
	require.ErrorIs(t, metricExporter.Shutdown(t.Context()), primaryErr)

	logExporter := multiLogExporter{
		primary: &recordingLogExporter{calls: &calls, name: "primary log", err: primaryErr},
		dev:     &recordingLogExporter{calls: &calls, name: "dev log", err: devErr},
	}
	require.ErrorIs(t, logExporter.Export(t.Context(), nil), primaryErr)
	require.ErrorIs(t, logExporter.ForceFlush(t.Context()), primaryErr)
	require.ErrorIs(t, logExporter.Shutdown(t.Context()), primaryErr)

	traceExporter := multiTraceExporter{
		primary: &recordingTraceExporter{calls: &calls, name: "primary trace", err: primaryErr},
		dev:     &recordingTraceExporter{calls: &calls, name: "dev trace", err: devErr},
	}
	require.ErrorIs(t, traceExporter.ExportSpans(t.Context(), nil), primaryErr)
	require.ErrorIs(t, traceExporter.Shutdown(t.Context()), primaryErr)

	require.Equal(t, []string{
		"primary metric export", "dev metric export",
		"primary metric force", "dev metric force",
		"primary metric shutdown", "dev metric shutdown",
		"primary log export", "dev log export",
		"primary log force", "dev log force",
		"primary log shutdown", "dev log shutdown",
		"primary trace export", "dev trace export",
		"primary trace shutdown", "dev trace shutdown",
	}, calls)
}

type recordingMetricExporter struct {
	calls *[]string
	name  string
	err   error
}

func (e *recordingMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.CumulativeTemporality
}

func (e *recordingMetricExporter) Aggregation(sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.AggregationDefault{}
}

func (e *recordingMetricExporter) Export(context.Context, *metricdata.ResourceMetrics) error {
	*e.calls = append(*e.calls, e.name+" export")
	return e.err
}

func (e *recordingMetricExporter) ForceFlush(context.Context) error {
	*e.calls = append(*e.calls, e.name+" force")
	return e.err
}

func (e *recordingMetricExporter) Shutdown(context.Context) error {
	*e.calls = append(*e.calls, e.name+" shutdown")
	return e.err
}

type recordingLogExporter struct {
	calls *[]string
	name  string
	err   error
}

func (e *recordingLogExporter) Export(context.Context, []sdklog.Record) error {
	*e.calls = append(*e.calls, e.name+" export")
	return e.err
}

func (e *recordingLogExporter) ForceFlush(context.Context) error {
	*e.calls = append(*e.calls, e.name+" force")
	return e.err
}

func (e *recordingLogExporter) Shutdown(context.Context) error {
	*e.calls = append(*e.calls, e.name+" shutdown")
	return e.err
}

type recordingTraceExporter struct {
	calls *[]string
	name  string
	err   error
}

func (e *recordingTraceExporter) ExportSpans(context.Context, []sdktrace.ReadOnlySpan) error {
	*e.calls = append(*e.calls, e.name+" export")
	return e.err
}

func (e *recordingTraceExporter) Shutdown(context.Context) error {
	*e.calls = append(*e.calls, e.name+" shutdown")
	return e.err
}
