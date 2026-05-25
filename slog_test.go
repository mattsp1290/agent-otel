package agentotel

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/mattsp1290/agent-otel/internal/otlptest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
)

func TestSlogBridgeExportsLogs(t *testing.T) {
	clearOTLPEnv(t)
	receiver := otlptest.Start(t)
	providers, shutdown, err := Init(t.Context(), Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		ServiceName:       "slog-test",
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
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

	logger := providers.SlogLogger("agent-otel/slog-test",
		WithSlogAttributes(attribute.String("component", "test")),
	)
	logger.InfoContext(t.Context(), "bridge message", slog.String("request_id", "abc123"))
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	record := waitForLogRecord(t, receiver)
	require.Equal(t, "bridge message", record.GetBody().GetStringValue())
	attrs := kvAttrs(record.GetAttributes())
	require.Equal(t, "abc123", attrs["request_id"].GetStringValue())

	snap := receiver.Snapshot()
	resourceAttrs := kvAttrs(snap.Logs[0].GetResourceLogs()[0].GetResource().GetAttributes())
	require.Equal(t, "slog-test", resourceAttrs["service.name"].GetStringValue())
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func TestSlogBridgeDisabledNoopDoesNotPanic(t *testing.T) {
	clearOTLPEnv(t)
	providers, shutdown, err := Init(t.Context(), Options{
		Enabled:           false,
		SkipGlobalInstall: true,
	})
	require.NoError(t, err)

	logger := providers.SlogLogger("agent-otel/noop")
	logger.InfoContext(t.Context(), "noop message", slog.String("request_id", "abc123"))
	require.NoError(t, shutdown.ForceFlush(t.Context()))
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func waitForLogRecord(t *testing.T, receiver *otlptest.Receiver) *logsv1.LogRecord {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	snap, err := receiver.WaitFor(ctx, func(s otlptest.Snapshot) bool {
		for _, req := range s.Logs {
			for _, resourceLogs := range req.GetResourceLogs() {
				for _, scopeLogs := range resourceLogs.GetScopeLogs() {
					if len(scopeLogs.GetLogRecords()) > 0 {
						return true
					}
				}
			}
		}
		return false
	})
	require.NoError(t, err)
	for _, req := range snap.Logs {
		for _, resourceLogs := range req.GetResourceLogs() {
			for _, scopeLogs := range resourceLogs.GetScopeLogs() {
				if records := scopeLogs.GetLogRecords(); len(records) > 0 {
					return records[0]
				}
			}
		}
	}
	require.FailNow(t, "log record not found")
	return nil
}
