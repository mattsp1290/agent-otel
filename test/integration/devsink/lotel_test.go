//go:build integration

package devsink

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/log"

	agentotel "github.com/mattsp1290/agent-otel"
	"github.com/mattsp1290/agent-otel/internal/otlptest"
	"github.com/mattsp1290/agent-otel/test/integration/lotelhelper"
)

func TestWithDevSinkExportsGenAIToLotel(t *testing.T) {
	lotel := lotelhelper.Start(t)
	if lotel == nil {
		return
	}
	primary := otlptest.Start(t)

	service := fmt.Sprintf("agent-otel-devsink-test-%d", time.Now().UnixNano())
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	providers, shutdown, err := agentotel.Init(ctx, agentotel.ApplyOptions(agentotel.Options{
		Enabled:           true,
		ServiceName:       service,
		SkipGlobalInstall: true,
		DialTimeout:       time.Second,
		BatchTimeout:      10 * time.Millisecond,
		ExportTimeout:     time.Second,
		TraceExporter: agentotel.ExporterConfig{
			Endpoint: primary.Endpoint(),
			Protocol: agentotel.ProtocolGRPC,
			Insecure: true,
		},
		MetricExporter: agentotel.ExporterConfig{
			Endpoint: primary.Endpoint(),
			Protocol: agentotel.ProtocolGRPC,
			Insecure: true,
		},
		LogExporter: agentotel.ExporterConfig{
			Endpoint: primary.Endpoint(),
			Protocol: agentotel.ProtocolGRPC,
			Insecure: true,
		},
	}, agentotel.WithDevSink(lotel.Endpoint())))
	require.NoError(t, err)

	ctx, span, err := agentotel.StartModelCall(ctx, providers.Tracer, agentotel.ModelCall{
		OperationName: agentotel.GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage: agentotel.Usage{
			InputTokens:  12,
			OutputTokens: 7,
			Available:    true,
		},
	})
	require.NoError(t, err)
	span.End()

	labels := agentotel.ModelMetricLabels{
		OperationName: agentotel.GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
	}
	require.NoError(t, providers.Instruments.RecordModelLatency(ctx, 0.125, labels))
	require.NoError(t, providers.Instruments.RecordUsage(ctx, agentotel.Usage{
		InputTokens:  12,
		OutputTokens: 7,
		Available:    true,
	}, labels))

	var rec log.Record
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(log.SeverityInfo)
	rec.SetBody(log.StringValue("devsink integration log"))
	providers.Logger.Emit(ctx, rec)

	require.NoError(t, shutdown.ForceFlush(ctx))
	require.NoError(t, shutdown.Shutdown(ctx))

	traces := lotel.WaitForTraces(t, service, 1, 20*time.Second)
	require.True(t, containsRecord(traces,
		agentotel.GenAIOperationChat,
		agentotel.AttrGenAIProviderName,
		"openai",
	), "trace records did not contain expected GenAI span attrs: %+v", traces)
	require.True(t, containsRecord(traces,
		agentotel.GenAIOperationChat,
		agentotel.AttrGenAIRequestModel,
		"gpt-4o",
	), "trace records did not contain expected request model: %+v", traces)

	metrics := lotel.WaitForMetrics(t, service, 1, 20*time.Second)
	require.True(t, containsRecord(metrics,
		agentotel.MetricGenAIClientOperationDuration,
		agentotel.AttrGenAIOperationName,
		agentotel.GenAIOperationChat,
	), "metric records did not contain expected latency metric attrs: %+v", metrics)

	logs := lotel.WaitForLogs(t, service, 1, 20*time.Second)
	require.True(t, containsRecord(logs, "devsink integration log", "", ""),
		"log records did not contain emitted log body: %+v", logs)
}

func containsRecord[T ~map[string]any](records []T, needles ...string) bool {
	for _, record := range records {
		all := true
		for _, needle := range needles {
			if needle == "" {
				continue
			}
			if !containsValue(any(record), needle) {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

func containsValue(v any, needle string) bool {
	switch x := v.(type) {
	case string:
		return x == needle
	case map[string]any:
		for key, value := range x {
			if key == needle || containsValue(value, needle) {
				return true
			}
		}
	case []any:
		for _, value := range x {
			if containsValue(value, needle) {
				return true
			}
		}
	case fmt.Stringer:
		return x.String() == needle
	}
	return fmt.Sprint(v) == needle
}
