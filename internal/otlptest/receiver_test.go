package otlptest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	logsvc "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	metricsvc "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	tracesvc "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	logsv1 "go.opentelemetry.io/proto/otlp/logs/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcev1 "go.opentelemetry.io/proto/otlp/resource/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestReceiverCapturesTraceMetricLogWireShape(t *testing.T) {
	t.Parallel()

	r := Start(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := r.Dial(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, conn.Close()) })

	_, err = tracesvc.NewTraceServiceClient(conn).Export(ctx, traceRequest())
	require.NoError(t, err)
	_, err = metricsvc.NewMetricsServiceClient(conn).Export(ctx, metricRequest())
	require.NoError(t, err)
	_, err = logsvc.NewLogsServiceClient(conn).Export(ctx, logRequest())
	require.NoError(t, err)

	snap, err := r.WaitFor(ctx, func(s Snapshot) bool {
		return len(s.Traces) == 1 && len(s.Metrics) == 1 && len(s.Logs) == 1
	})
	require.NoError(t, err)

	spanResource := snap.Traces[0].GetResourceSpans()[0].GetResource()
	require.Equal(t, []string{"service.name"}, AttributeKeys(spanResource.GetAttributes()))
	span := snap.Traces[0].GetResourceSpans()[0].GetScopeSpans()[0].GetSpans()[0]
	require.Equal(t, "chat gpt-4o", span.GetName())
	require.Equal(t, []string{"gen_ai.request.model"}, AttributeKeys(span.GetAttributes()))

	metricResource := snap.Metrics[0].GetResourceMetrics()[0].GetResource()
	require.Equal(t, []string{"service.name"}, AttributeKeys(metricResource.GetAttributes()))
	metric := snap.Metrics[0].GetResourceMetrics()[0].GetScopeMetrics()[0].GetMetrics()[0]
	require.Equal(t, "gen_ai.client.token.usage", metric.GetName())
	point := metric.GetGauge().GetDataPoints()[0]
	require.Equal(t, []string{"gen_ai.token.type"}, AttributeKeys(point.GetAttributes()))

	logResource := snap.Logs[0].GetResourceLogs()[0].GetResource()
	require.Equal(t, []string{"service.name"}, AttributeKeys(logResource.GetAttributes()))
	record := snap.Logs[0].GetResourceLogs()[0].GetScopeLogs()[0].GetLogRecords()[0]
	require.Equal(t, []string{"gen_ai.operation.name"}, AttributeKeys(record.GetAttributes()))
}

func traceRequest() *tracesvc.ExportTraceServiceRequest {
	return &tracesvc.ExportTraceServiceRequest{
		ResourceSpans: []*tracev1.ResourceSpans{
			{
				Resource: resource("agent-otel-test"),
				ScopeSpans: []*tracev1.ScopeSpans{
					{
						Spans: []*tracev1.Span{
							{
								Name:       "chat gpt-4o",
								TraceId:    []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
								SpanId:     []byte{1, 2, 3, 4, 5, 6, 7, 8},
								Attributes: []*commonv1.KeyValue{stringAttr("gen_ai.request.model", "gpt-4o")},
							},
						},
					},
				},
			},
		},
	}
}

func metricRequest() *metricsvc.ExportMetricsServiceRequest {
	return &metricsvc.ExportMetricsServiceRequest{
		ResourceMetrics: []*metricsv1.ResourceMetrics{
			{
				Resource: resource("agent-otel-test"),
				ScopeMetrics: []*metricsv1.ScopeMetrics{
					{
						Metrics: []*metricsv1.Metric{
							{
								Name: "gen_ai.client.token.usage",
								Data: &metricsv1.Metric_Gauge{
									Gauge: &metricsv1.Gauge{
										DataPoints: []*metricsv1.NumberDataPoint{
											{
												Attributes: []*commonv1.KeyValue{stringAttr("gen_ai.token.type", "input")},
												Value: &metricsv1.NumberDataPoint_AsInt{
													AsInt: 100,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func logRequest() *logsvc.ExportLogsServiceRequest {
	return &logsvc.ExportLogsServiceRequest{
		ResourceLogs: []*logsv1.ResourceLogs{
			{
				Resource: resource("agent-otel-test"),
				ScopeLogs: []*logsv1.ScopeLogs{
					{
						LogRecords: []*logsv1.LogRecord{
							{
								Attributes: []*commonv1.KeyValue{stringAttr("gen_ai.operation.name", "chat")},
								Body:       stringValue("model call"),
							},
						},
					},
				},
			},
		},
	}
}

func resource(serviceName string) *resourcev1.Resource {
	return &resourcev1.Resource{
		Attributes: []*commonv1.KeyValue{stringAttr("service.name", serviceName)},
	}
}

func stringAttr(key, value string) *commonv1.KeyValue {
	return &commonv1.KeyValue{
		Key:   key,
		Value: stringValue(value),
	}
}

func stringValue(value string) *commonv1.AnyValue {
	return &commonv1.AnyValue{
		Value: &commonv1.AnyValue_StringValue{StringValue: value},
	}
}
