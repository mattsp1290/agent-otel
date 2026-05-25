package agentotel

import (
	"testing"
	"time"

	"github.com/mattsp1290/agent-otel/internal/otlptest"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestOpenLLMetryDefaultIsNativeOnly(t *testing.T) {
	providers, shutdown, receiver := startSpanTestProviders(t)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{InputTokens: 100, OutputTokens: 180, Available: true},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	attrs := spanAttrs(wireSpan)
	require.Equal(t, "gpt-4o", attrs[AttrGenAIRequestModel].GetStringValue())
	require.Equal(t, "openai", attrs[AttrGenAIProviderName].GetStringValue())
	require.Equal(t, GenAIOperationChat, attrs[AttrGenAIOperationName].GetStringValue())
	require.NotContains(t, attrs, AttrLLMRequestType)
	require.NotContains(t, attrs, AttrLLMUsageTotalTokens)
	require.NotContains(t, attrs, AttrTraceloopSpanKind)
	require.NotContains(t, attrs, AttrTraceloopWorkflow)
}

func TestOpenLLMetryCompatAddsLegacyAttrs(t *testing.T) {
	providers, shutdown, receiver := startOpenLLMetryProviders(t, OpenLLMetryCompatOptions{
		WorkflowName:  "advise",
		EntityName:    "advisor",
		EntityPath:    "advisor/core",
		EntityVersion: "v1",
		SpanKind:      TraceloopWorkflow,
		AssociationProperties: map[string]string{
			"entrypoint": "cli",
		},
	})

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{InputTokens: 100, OutputTokens: 180, Available: true},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	attrs := spanAttrs(wireSpan)
	require.Equal(t, GenAIOperationChat, attrs[AttrGenAIOperationName].GetStringValue())
	require.Equal(t, "openai", attrs[AttrGenAIProviderName].GetStringValue())
	require.Equal(t, "gpt-4o", attrs[AttrGenAIRequestModel].GetStringValue())
	require.Equal(t, "chat", attrs[AttrLLMRequestType].GetStringValue())
	require.Equal(t, int64(280), attrs[AttrLLMUsageTotalTokens].GetIntValue())
	require.Equal(t, "workflow", attrs[AttrTraceloopSpanKind].GetStringValue())
	require.Equal(t, "advise", attrs[AttrTraceloopWorkflow].GetStringValue())
	require.Equal(t, "advisor", attrs[AttrTraceloopEntityName].GetStringValue())
	require.Equal(t, "advisor/core", attrs[AttrTraceloopEntityPath].GetStringValue())
	require.Equal(t, "v1", attrs[AttrTraceloopEntityVer].GetStringValue())
	require.Equal(t, `{"entrypoint":"cli"}`, attrs[AttrTraceloopAssocProps].GetStringValue())
	require.NotContains(t, attrs, AttrGenAIInputMessages)
	require.NotContains(t, attrs, AttrGenAIOutputMessages)
}

func TestOpenLLMetryCompatTextCompletionAndUnavailableUsage(t *testing.T) {
	providers, shutdown, receiver := startOpenLLMetryProviders(t, OpenLLMetryCompatOptions{})

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationTextCompletion,
		ProviderName:  "openai",
		RequestModel:  "gpt-3.5",
		Usage:         Usage{Available: false},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "text_completion gpt-3.5")
	attrs := spanAttrs(wireSpan)
	require.Equal(t, "completion", attrs[AttrLLMRequestType].GetStringValue())
	require.NotContains(t, attrs, AttrLLMUsageTotalTokens)
	require.NotContains(t, attrs, AttrTraceloopSpanKind)
}

func TestOpenLLMetryCompatUnknownOperationIsBounded(t *testing.T) {
	attrs := openLLMetryModelCallAttributes(ModelCall{
		OperationName: "provider_specific_operation",
		Usage:         Usage{Available: false},
	}, &OpenLLMetryCompatOptions{})
	require.Contains(t, attrs, attributeString(AttrLLMRequestType, LLMRequestTypeUnknown))
}

func startOpenLLMetryProviders(t *testing.T, compat OpenLLMetryCompatOptions) (*Providers, *Shutdown, *otlptest.Receiver) {
	t.Helper()
	clearOTLPEnv(t)
	receiver := otlptest.Start(t)
	providers, shutdown, err := Init(t.Context(), ApplyOptions(Options{
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
	}, WithOpenLLMetryCompat(compat)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = shutdown.Shutdown(t.Context()) })
	return providers, shutdown, receiver
}

func attributeString(key, value string) attribute.KeyValue {
	return attribute.String(key, value)
}
