package otlptest

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	logsv1 "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Receiver struct {
	t testing.TB

	server *grpc.Server
	addr   string

	mu      sync.Mutex
	traces  []*tracev1.ExportTraceServiceRequest
	metrics []*metricsv1.ExportMetricsServiceRequest
	logs    []*logsv1.ExportLogsServiceRequest
}

type Snapshot struct {
	Traces  []*tracev1.ExportTraceServiceRequest
	Metrics []*metricsv1.ExportMetricsServiceRequest
	Logs    []*logsv1.ExportLogsServiceRequest
}

func Start(t testing.TB) *Receiver {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("otlptest: listen: %v", err)
	}

	r := &Receiver{
		t:      t,
		server: grpc.NewServer(),
		addr:   ln.Addr().String(),
	}

	tracev1.RegisterTraceServiceServer(r.server, traceService{receiver: r})
	metricsv1.RegisterMetricsServiceServer(r.server, metricService{receiver: r})
	logsv1.RegisterLogsServiceServer(r.server, logService{receiver: r})

	go func() {
		if err := r.server.Serve(ln); err != nil {
			t.Logf("otlptest: serve stopped: %v", err)
		}
	}()
	t.Cleanup(r.Shutdown)

	return r
}

func (r *Receiver) Endpoint() string {
	return r.addr
}

func (r *Receiver) Dial(ctx context.Context) (*grpc.ClientConn, error) {
	return grpc.NewClient(r.addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", r.addr)
		}),
	)
}

func (r *Receiver) Shutdown() {
	r.server.Stop()
}

func (r *Receiver) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return Snapshot{
		Traces:  append([]*tracev1.ExportTraceServiceRequest(nil), r.traces...),
		Metrics: append([]*metricsv1.ExportMetricsServiceRequest(nil), r.metrics...),
		Logs:    append([]*logsv1.ExportLogsServiceRequest(nil), r.logs...),
	}
}

func (r *Receiver) WaitFor(ctx context.Context, want func(Snapshot) bool) (Snapshot, error) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		snap := r.Snapshot()
		if want(snap) {
			return snap, nil
		}

		select {
		case <-ctx.Done():
			return snap, fmt.Errorf("otlptest: wait: %w", ctx.Err())
		case <-ticker.C:
		}
	}
}

func AttributeKeys(attrs []*commonv1.KeyValue) []string {
	keys := make([]string, 0, len(attrs))
	for _, attr := range attrs {
		keys = append(keys, attr.GetKey())
	}
	return keys
}

func (r *Receiver) appendTrace(req *tracev1.ExportTraceServiceRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.traces = append(r.traces, req)
}

func (r *Receiver) appendMetric(req *metricsv1.ExportMetricsServiceRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.metrics = append(r.metrics, req)
}

func (r *Receiver) appendLog(req *logsv1.ExportLogsServiceRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.logs = append(r.logs, req)
}

type traceService struct {
	tracev1.UnimplementedTraceServiceServer
	receiver *Receiver
}

func (s traceService) Export(_ context.Context, req *tracev1.ExportTraceServiceRequest) (*tracev1.ExportTraceServiceResponse, error) {
	s.receiver.appendTrace(req)
	return &tracev1.ExportTraceServiceResponse{}, nil
}

type metricService struct {
	metricsv1.UnimplementedMetricsServiceServer
	receiver *Receiver
}

func (s metricService) Export(_ context.Context, req *metricsv1.ExportMetricsServiceRequest) (*metricsv1.ExportMetricsServiceResponse, error) {
	s.receiver.appendMetric(req)
	return &metricsv1.ExportMetricsServiceResponse{}, nil
}

type logService struct {
	logsv1.UnimplementedLogsServiceServer
	receiver *Receiver
}

func (s logService) Export(_ context.Context, req *logsv1.ExportLogsServiceRequest) (*logsv1.ExportLogsServiceResponse, error) {
	s.receiver.appendLog(req)
	return &logsv1.ExportLogsServiceResponse{}, nil
}
