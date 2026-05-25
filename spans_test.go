package agentotel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	commonv1 "go.opentelemetry.io/proto/otlp/common/v1"
	tracev1 "go.opentelemetry.io/proto/otlp/trace/v1"

	"github.com/mattsp1290/agent-otel/internal/otlptest"
)

func TestStartModelCallEmitsGenAIAttrsUsageAndProductAttrs(t *testing.T) {
	providers, shutdown, receiver := startSpanTestProviders(t)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		System:        "openai",
		RequestModel:  "gpt-4o",
		ResponseModel: "gpt-4o-2024-08-06",
		Usage:         Usage{InputTokens: 100, OutputTokens: 180, Available: true},
		Attributes: []attribute.KeyValue{
			attribute.String("advisor.entrypoint", "cli"),
		},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	attrs := spanAttrs(wireSpan)
	require.Equal(t, "chat gpt-4o", wireSpan.GetName())
	require.Equal(t, "chat", attrs[AttrGenAIOperationName].GetStringValue())
	require.Equal(t, "openai", attrs[AttrGenAIProviderName].GetStringValue())
	require.Equal(t, "openai", attrs[AttrGenAISystem].GetStringValue())
	require.Equal(t, "gpt-4o", attrs[AttrGenAIRequestModel].GetStringValue())
	require.Equal(t, "gpt-4o-2024-08-06", attrs[AttrGenAIResponseModel].GetStringValue())
	require.Equal(t, int64(100), attrs[AttrGenAIUsageInputTokens].GetIntValue())
	require.Equal(t, int64(180), attrs[AttrGenAIUsageOutputTokens].GetIntValue())
	require.True(t, attrs[AttrAgentOTelUsageAvailable].GetBoolValue())
	require.Equal(t, "cli", attrs["advisor.entrypoint"].GetStringValue())
}

func TestStartModelCallUnavailableUsageOmitsTokenAttrs(t *testing.T) {
	providers, shutdown, receiver := startSpanTestProviders(t)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	attrs := spanAttrs(wireSpan)
	require.False(t, attrs[AttrAgentOTelUsageAvailable].GetBoolValue())
	require.NotContains(t, attrs, AttrGenAIUsageInputTokens)
	require.NotContains(t, attrs, AttrGenAIUsageOutputTokens)
}

func TestStartToolAndAgentSpansEmitGenAIAttrs(t *testing.T) {
	providers, shutdown, receiver := startSpanTestProviders(t)

	_, agentSpan, err := providers.StartAgentOperation(t.Context(), AgentOperation{
		OperationName: GenAIOperationInvokeAgent,
		AgentName:     "advisor",
		Attributes:    []attribute.KeyValue{attribute.String("advisor.entrypoint", "mcp")},
	})
	require.NoError(t, err)
	agentSpan.End()

	_, toolSpan, err := providers.StartToolCall(t.Context(), ToolCall{
		ToolName:   "search",
		Attributes: []attribute.KeyValue{attribute.String("tool.outcome", "succeeded")},
	})
	require.NoError(t, err)
	toolSpan.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	agentWireSpan := waitForSpan(t, receiver, "invoke_agent advisor")
	agentAttrs := spanAttrs(agentWireSpan)
	require.Equal(t, GenAIOperationInvokeAgent, agentAttrs[AttrGenAIOperationName].GetStringValue())
	require.Equal(t, "advisor", agentAttrs[AttrGenAIAgentName].GetStringValue())
	require.Equal(t, "mcp", agentAttrs["advisor.entrypoint"].GetStringValue())

	toolWireSpan := waitForSpan(t, receiver, "execute_tool search")
	toolAttrs := spanAttrs(toolWireSpan)
	require.Equal(t, GenAIOperationExecuteTool, toolAttrs[AttrGenAIOperationName].GetStringValue())
	require.Equal(t, "search", toolAttrs[AttrGenAIToolName].GetStringValue())
	require.Equal(t, "succeeded", toolAttrs["tool.outcome"].GetStringValue())
}

func TestSpanHelpersEmitErrorAndFallbackAttrs(t *testing.T) {
	providers, shutdown, receiver := startSpanTestProviders(t)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
	})
	require.NoError(t, err)
	AddFallbackEvent(span, Fallback{
		ProviderName: "openai",
		FromProvider: "openai",
		ToProvider:   "anthropic",
	})
	MarkSpanError(span, errors.New("provider failed"), "provider_error")
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	attrs := spanAttrs(wireSpan)
	require.Equal(t, "provider_error", attrs[AttrErrorType].GetStringValue())
	require.Equal(t, tracev1.Status_STATUS_CODE_ERROR, wireSpan.GetStatus().GetCode())
	require.Len(t, wireSpan.GetEvents(), 2)

	fallbackEvent := findSpanEvent(t, wireSpan, MetricAgentOTelFallbackEngaged)
	eventAttrs := kvAttrs(fallbackEvent.GetAttributes())
	require.Equal(t, "openai", eventAttrs[AttrGenAIProviderName].GetStringValue())
	require.Equal(t, "openai", eventAttrs[AttrAgentOTelProviderFrom].GetStringValue())
	require.Equal(t, "anthropic", eventAttrs[AttrAgentOTelProviderTo].GetStringValue())
}

func TestStartModelCallRejectsNegativeUsage(t *testing.T) {
	providers, _, _ := startSpanTestProviders(t)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		RequestModel:  "gpt-4o",
		Usage:         Usage{InputTokens: -1, Available: true},
	})
	require.ErrorIs(t, err, ErrNegativeUsage)
	require.Nil(t, span)
}

func startSpanTestProviders(t *testing.T) (*Providers, *Shutdown, *otlptest.Receiver) {
	t.Helper()
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
	t.Cleanup(func() { _ = shutdown.Shutdown(context.Background()) })
	return providers, shutdown, receiver
}

func waitForSpan(t *testing.T, receiver *otlptest.Receiver, name string) *tracev1.Span {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	snap, err := receiver.WaitFor(ctx, func(s otlptest.Snapshot) bool {
		return findSpan(s, name) != nil
	})
	require.NoError(t, err)
	span := findSpan(snap, name)
	require.NotNil(t, span)
	return span
}

func findSpan(snap otlptest.Snapshot, name string) *tracev1.Span {
	for _, req := range snap.Traces {
		for _, resourceSpans := range req.GetResourceSpans() {
			for _, scopeSpans := range resourceSpans.GetScopeSpans() {
				for _, span := range scopeSpans.GetSpans() {
					if span.GetName() == name {
						return span
					}
				}
			}
		}
	}
	return nil
}

func findSpanEvent(t *testing.T, span *tracev1.Span, name string) *tracev1.Span_Event {
	t.Helper()
	for _, event := range span.GetEvents() {
		if event.GetName() == name {
			return event
		}
	}
	require.FailNow(t, "span event not found", name)
	return nil
}

func spanAttrs(span *tracev1.Span) map[string]*commonv1.AnyValue {
	return kvAttrs(span.GetAttributes())
}

func kvAttrs(attrs []*commonv1.KeyValue) map[string]*commonv1.AnyValue {
	out := make(map[string]*commonv1.AnyValue, len(attrs))
	for _, attr := range attrs {
		out[attr.GetKey()] = attr.GetValue()
	}
	return out
}
