package agentotel

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/mattsp1290/agent-otel/internal/otlptest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/metric"
	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/protobuf/proto"
)

func TestInitExportsAllSignalsOverGRPC(t *testing.T) {
	clearOTLPEnv(t)
	receiver := otlptest.Start(t)

	providers, shutdown, err := Init(t.Context(), Options{
		Enabled:           true,
		SkipGlobalInstall: true,
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
	require.NotNil(t, providers)
	require.NotNil(t, shutdown)

	emitTestTelemetry(t, providers)
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	snap, err := receiver.WaitFor(ctx, func(s otlptest.Snapshot) bool {
		return len(s.Traces) > 0 && len(s.Metrics) > 0 && len(s.Logs) > 0
	})
	require.NoError(t, err)
	require.NotEmpty(t, snap.Traces[0].GetResourceSpans())
	require.NotEmpty(t, snap.Metrics[0].GetResourceMetrics())
	require.NotEmpty(t, snap.Logs[0].GetResourceLogs())
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func TestInitExportsAllSignalsOverHTTP(t *testing.T) {
	clearOTLPEnv(t)
	receiver := startHTTPOTLPReceiver(t)

	providers, shutdown, err := Init(t.Context(), Options{
		Enabled:           true,
		SkipGlobalInstall: true,
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		TraceExporter: ExporterConfig{
			Endpoint: receiver.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		MetricExporter: ExporterConfig{
			Endpoint: receiver.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
		LogExporter: ExporterConfig{
			Endpoint: receiver.URL(),
			Protocol: ProtocolHTTPProtobuf,
		},
	})
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NotNil(t, shutdown)

	emitTestTelemetry(t, providers)
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	snap := receiver.WaitFor(t, func(s httpOTLPSnapshot) bool {
		return len(s.Traces) > 0 && len(s.Metrics) > 0 && len(s.Logs) > 0
	})
	require.NotEmpty(t, snap.Traces[0].GetResourceSpans())
	require.NotEmpty(t, snap.Metrics[0].GetResourceMetrics())
	require.NotEmpty(t, snap.Logs[0].GetResourceLogs())
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func TestInitUnreachableExporterFallsBackToNoop(t *testing.T) {
	clearOTLPEnv(t)

	providers, shutdown, err := Init(t.Context(), Options{
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
	})
	require.NoError(t, err)
	require.NotNil(t, providers)
	require.NotNil(t, shutdown)
	emitTestTelemetry(t, providers)
	require.NoError(t, shutdown.ForceFlush(t.Context()))
	require.NoError(t, shutdown.Shutdown(t.Context()))
}

func emitTestTelemetry(t *testing.T, providers *Providers) {
	t.Helper()
	ctx := t.Context()

	_, span := providers.Tracer.Start(ctx, "agent-otel-test-span")
	span.End()

	counter, err := providers.Meter.Int64Counter("agent_otel.test.counter")
	require.NoError(t, err)
	counter.Add(ctx, 1, metric.WithAttributes())

	var record log.Record
	record.SetSeverity(log.SeverityInfo)
	record.SetBody(log.StringValue("agent-otel-test-log"))
	record.SetTimestamp(time.Now())
	providers.Logger.Emit(ctx, record)
}

type httpOTLPReceiver struct {
	server *httptest.Server

	mu      sync.Mutex
	traces  []*tracev1.ExportTraceServiceRequest
	metrics []*metricsv1.ExportMetricsServiceRequest
	logs    []*logsv1.ExportLogsServiceRequest
}

type httpOTLPSnapshot struct {
	Traces  []*tracev1.ExportTraceServiceRequest
	Metrics []*metricsv1.ExportMetricsServiceRequest
	Logs    []*logsv1.ExportLogsServiceRequest
}

func startHTTPOTLPReceiver(t *testing.T) *httpOTLPReceiver {
	t.Helper()
	receiver := &httpOTLPReceiver{}
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/traces", receiver.handleTraces)
	mux.HandleFunc("/v1/metrics", receiver.handleMetrics)
	mux.HandleFunc("/v1/logs", receiver.handleLogs)
	receiver.server = httptest.NewServer(mux)
	t.Cleanup(receiver.server.Close)
	return receiver
}

func (r *httpOTLPReceiver) URL() string {
	return r.server.URL
}

func (r *httpOTLPReceiver) Snapshot() httpOTLPSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return httpOTLPSnapshot{
		Traces:  append([]*tracev1.ExportTraceServiceRequest(nil), r.traces...),
		Metrics: append([]*metricsv1.ExportMetricsServiceRequest(nil), r.metrics...),
		Logs:    append([]*logsv1.ExportLogsServiceRequest(nil), r.logs...),
	}
}

func (r *httpOTLPReceiver) WaitFor(t *testing.T, want func(httpOTLPSnapshot) bool) httpOTLPSnapshot {
	t.Helper()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		snap := r.Snapshot()
		if want(snap) {
			return snap
		}
		select {
		case <-deadline:
			require.FailNow(t, "timed out waiting for HTTP OTLP export")
		case <-ticker.C:
		}
	}
}

func (r *httpOTLPReceiver) handleTraces(w http.ResponseWriter, req *http.Request) {
	var exportReq tracev1.ExportTraceServiceRequest
	if decodeOTLPHTTP(w, req, &exportReq) {
		r.mu.Lock()
		r.traces = append(r.traces, &exportReq)
		r.mu.Unlock()
	}
}

func (r *httpOTLPReceiver) handleMetrics(w http.ResponseWriter, req *http.Request) {
	var exportReq metricsv1.ExportMetricsServiceRequest
	if decodeOTLPHTTP(w, req, &exportReq) {
		r.mu.Lock()
		r.metrics = append(r.metrics, &exportReq)
		r.mu.Unlock()
	}
}

func (r *httpOTLPReceiver) handleLogs(w http.ResponseWriter, req *http.Request) {
	var exportReq logsv1.ExportLogsServiceRequest
	if decodeOTLPHTTP(w, req, &exportReq) {
		r.mu.Lock()
		r.logs = append(r.logs, &exportReq)
		r.mu.Unlock()
	}
}

func decodeOTLPHTTP(w http.ResponseWriter, req *http.Request, msg proto.Message) bool {
	if req.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return false
	}
	body, err := io.ReadAll(req.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if err := proto.Unmarshal(body, msg); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	w.WriteHeader(http.StatusOK)
	return true
}
