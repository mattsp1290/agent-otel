package agentotel

import (
	"context"
	"errors"
)

const AttrAgentOTelRedactionFailed = "agent_otel.redaction.failed"

var ErrRedactionFailed = errors.New("agentotel: prompt redaction failed")

type PayloadKind string

const (
	PayloadPrompt             PayloadKind = "prompt"
	PayloadCompletion         PayloadKind = "completion"
	PayloadSystemInstructions PayloadKind = "system_instructions"
	PayloadToolDefinitions    PayloadKind = "tool_definitions"
)

type PromptPayload struct {
	Kind       PayloadKind
	Provider   string
	Model      string
	Operation  string
	Attributes map[string]any
	Value      any
}

type PromptRedactor interface {
	RedactPrompt(ctx context.Context, payload PromptPayload) (PromptPayload, error)
}

type RedactionFailureMode int

const (
	DropPayloadOnRedactionError RedactionFailureMode = iota
	ReturnErrorOnRedactionError
)

type PayloadCaptureOptions struct {
	Redactor    PromptRedactor
	FailureMode RedactionFailureMode
}

type PayloadCaptureOption func(*PayloadCaptureOptions)

func WithPayloadCapture(redactor PromptRedactor, options ...PayloadCaptureOption) Option {
	return func(opts *Options) {
		capture := PayloadCaptureOptions{Redactor: redactor}
		if capture.Redactor == nil {
			capture.Redactor = DropAllPayloadsRedactor{}
		}
		for _, opt := range options {
			if opt != nil {
				opt(&capture)
			}
		}
		opts.PayloadCapture = &capture
	}
}

func WithRedactionFailureMode(mode RedactionFailureMode) PayloadCaptureOption {
	return func(opts *PayloadCaptureOptions) {
		opts.FailureMode = mode
	}
}

type DropAllPayloadsRedactor struct{}

func (DropAllPayloadsRedactor) RedactPrompt(_ context.Context, payload PromptPayload) (PromptPayload, error) {
	payload.Value = nil
	payload.Attributes = nil
	return payload, nil
}

type AllowUnredactedPayloadsRedactor struct{}

func (AllowUnredactedPayloadsRedactor) RedactPrompt(_ context.Context, payload PromptPayload) (PromptPayload, error) {
	return payload, nil
}
