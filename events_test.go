package agentotel

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/mattsp1290/agent-otel/internal/otlptest"
)

const secretPrompt = "SECRET_PROMPT_DO_NOT_EXPORT"

func TestPayloadCaptureDisabledByDefault(t *testing.T) {
	providers, shutdown, receiver := startSpanTestProviders(t)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
		Payloads: []PromptPayload{
			{Kind: PayloadPrompt, Value: secretPrompt},
		},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	require.Empty(t, wireSpan.GetEvents())
	require.NotContains(t, spanAttrs(wireSpan), AttrGenAIInputMessages)
}

func TestPayloadCaptureRedactsBeforeAttachingEvent(t *testing.T) {
	redactor := &recordingRedactor{value: "[REDACTED]"}
	providers, shutdown, receiver := startPayloadTestProviders(t, WithPayloadCapture(redactor))

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
		Payloads: []PromptPayload{
			{Kind: PayloadPrompt, Value: secretPrompt},
			{Kind: PayloadCompletion, Value: map[string]string{"text": "safe completion"}},
		},
	})
	require.NoError(t, err)
	require.True(t, redactor.called)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	event := findSpanEvent(t, wireSpan, EventGenAIClientInferenceOperationDetails)
	attrs := kvAttrs(event.GetAttributes())
	require.Equal(t, "[REDACTED]", attrs[AttrGenAIInputMessages].GetStringValue())
	require.Equal(t, "[REDACTED]", attrs[AttrGenAIOutputMessages].GetStringValue())
	require.Equal(t, GenAIOperationChat, attrs[AttrGenAIOperationName].GetStringValue())
	require.Equal(t, "openai", attrs[AttrGenAIProviderName].GetStringValue())
	require.NotContains(t, attrs[AttrGenAIInputMessages].GetStringValue(), secretPrompt)
}

func TestPayloadCaptureRedactorErrorDropsEventByDefault(t *testing.T) {
	providers, shutdown, receiver := startPayloadTestProviders(t, WithPayloadCapture(errorRedactor{}))

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
		Payloads: []PromptPayload{
			{Kind: PayloadPrompt, Value: secretPrompt},
		},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	attrs := spanAttrs(wireSpan)
	require.True(t, attrs[AttrAgentOTelRedactionFailed].GetBoolValue())
	require.Empty(t, wireSpan.GetEvents())
	require.NotContains(t, attrs, AttrGenAIInputMessages)
}

func TestPayloadCaptureStrictRedactorErrorReturnsError(t *testing.T) {
	providers, shutdown, receiver := startPayloadTestProviders(t, WithPayloadCapture(errorRedactor{}, WithRedactionFailureMode(ReturnErrorOnRedactionError)))

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
		Payloads: []PromptPayload{
			{Kind: PayloadPrompt, Value: secretPrompt},
		},
	})
	require.ErrorIs(t, err, ErrRedactionFailed)
	require.NotNil(t, span)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	require.True(t, spanAttrs(wireSpan)[AttrAgentOTelRedactionFailed].GetBoolValue())
	require.Empty(t, wireSpan.GetEvents())
}

func TestPayloadCaptureUnrelatedOptionsDoNotEnableCapture(t *testing.T) {
	providers, shutdown, receiver := startPayloadTestProviders(t,
		WithDatadogPreset(),
		WithOpenLLMetryCompat(OpenLLMetryCompatOptions{SpanKind: TraceloopWorkflow}),
	)

	_, span, err := providers.StartModelCall(t.Context(), ModelCall{
		OperationName: GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage:         Usage{Available: false},
		Payloads: []PromptPayload{
			{Kind: PayloadPrompt, Value: secretPrompt},
		},
	})
	require.NoError(t, err)
	span.End()
	require.NoError(t, shutdown.ForceFlush(t.Context()))

	wireSpan := waitForSpan(t, receiver, "chat gpt-4o")
	require.Empty(t, wireSpan.GetEvents())
	attrs := spanAttrs(wireSpan)
	require.Equal(t, "chat", attrs[AttrLLMRequestType].GetStringValue())
	require.NotContains(t, attrs, AttrGenAIInputMessages)
}

func startPayloadTestProviders(t *testing.T, options ...Option) (*Providers, *Shutdown, *otlptest.Receiver) {
	t.Helper()
	clearOTLPEnv(t)
	receiver := otlptest.Start(t)
	base := Options{
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
	}
	providers, shutdown, err := Init(t.Context(), ApplyOptions(base, options...))
	require.NoError(t, err)
	t.Cleanup(func() { _ = shutdown.Shutdown(context.Background()) })
	return providers, shutdown, receiver
}

type recordingRedactor struct {
	value  any
	called bool
}

func (r *recordingRedactor) RedactPrompt(_ context.Context, payload PromptPayload) (PromptPayload, error) {
	r.called = true
	payload.Value = r.value
	return payload, nil
}

type errorRedactor struct{}

func (errorRedactor) RedactPrompt(context.Context, PromptPayload) (PromptPayload, error) {
	return PromptPayload{}, errors.New("redactor failed on " + secretPrompt)
}
