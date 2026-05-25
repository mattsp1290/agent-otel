package agentotel

import (
	"context"
	"encoding/json"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func attachPromptPayloadEvent(ctx context.Context, span trace.Span, call ModelCall, capture *PayloadCaptureOptions) error {
	if span == nil || capture == nil || capture.Redactor == nil || len(call.Payloads) == 0 {
		return nil
	}

	attrs := []attribute.KeyValue{}
	for _, payload := range call.Payloads {
		redacted, err := capture.Redactor.RedactPrompt(ctx, payload)
		if err != nil {
			span.SetAttributes(attribute.Bool(AttrAgentOTelRedactionFailed, true))
			if capture.FailureMode == ReturnErrorOnRedactionError {
				return fmt.Errorf("%w", ErrRedactionFailed)
			}
			return nil
		}
		attr, ok := redactedPayloadAttribute(redacted)
		if ok {
			attrs = append(attrs, attr)
		}
	}

	if len(attrs) == 0 {
		return nil
	}
	attrs = append(attrs, payloadCorrelationAttrs(call)...)
	span.AddEvent(EventGenAIClientInferenceOperationDetails, trace.WithAttributes(attrs...))
	return nil
}

func redactedPayloadAttribute(payload PromptPayload) (attribute.KeyValue, bool) {
	key := payloadAttrKey(payload.Kind)
	if key == "" || payload.Value == nil {
		return attribute.KeyValue{}, false
	}
	value, ok := payloadValueString(payload.Value)
	if !ok {
		return attribute.KeyValue{}, false
	}
	return attribute.String(key, value), true
}

func payloadAttrKey(kind PayloadKind) string {
	switch kind {
	case PayloadPrompt:
		return AttrGenAIInputMessages
	case PayloadCompletion:
		return AttrGenAIOutputMessages
	case PayloadSystemInstructions:
		return AttrGenAISystemInstructions
	case PayloadToolDefinitions:
		return AttrGenAIToolDefinitions
	default:
		return ""
	}
}

func payloadValueString(value any) (string, bool) {
	switch v := value.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return "", false
		}
		return string(encoded), true
	}
}

func payloadCorrelationAttrs(call ModelCall) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 3)
	if call.OperationName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIOperationName, call.OperationName))
	}
	if call.ProviderName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIProviderName, call.ProviderName))
	}
	if call.RequestModel != "" {
		attrs = append(attrs, attribute.String(AttrGenAIRequestModel, call.RequestModel))
	}
	return attrs
}
