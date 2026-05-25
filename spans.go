package agentotel

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

var ErrNilTracer = errors.New("agentotel: tracer is nil")

// ModelCall describes a GenAI model operation span.
type ModelCall struct {
	OperationName string
	ProviderName  string
	System        string
	RequestModel  string
	ResponseModel string
	Usage         Usage
	Attributes    []attribute.KeyValue
	Payloads      []PromptPayload
}

// AgentOperation describes a GenAI agent operation span.
type AgentOperation struct {
	OperationName string
	AgentName     string
	Attributes    []attribute.KeyValue
}

// ToolCall describes a GenAI tool execution span.
type ToolCall struct {
	ToolName   string
	Attributes []attribute.KeyValue
}

// Fallback describes a provider fallback event or span annotation.
type Fallback struct {
	ProviderName string
	FromProvider string
	ToProvider   string
	Attributes   []attribute.KeyValue
}

// StartModelCall starts a GenAI model-call span with standard attributes.
func StartModelCall(ctx context.Context, tracer trace.Tracer, call ModelCall, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	return startModelCall(ctx, tracer, call, nil, nil, opts...)
}

func startModelCall(ctx context.Context, tracer trace.Tracer, call ModelCall, compat *OpenLLMetryCompatOptions, capture *PayloadCaptureOptions, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	if tracer == nil {
		return ctx, nil, ErrNilTracer
	}
	attrs, err := modelCallAttributes(call)
	if err != nil {
		return ctx, nil, err
	}
	attrs = append(attrs, openLLMetryModelCallAttributes(call, compat)...)
	opts = append([]trace.SpanStartOption{trace.WithAttributes(attrs...)}, opts...)
	spanCtx, span := tracer.Start(ctx, modelCallSpanName(call), opts...)
	if err := attachPromptPayloadEvent(ctx, span, call, capture); err != nil {
		return spanCtx, span, err
	}
	return spanCtx, span, nil
}

// StartAgentOperation starts a GenAI agent operation span.
func StartAgentOperation(ctx context.Context, tracer trace.Tracer, op AgentOperation, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	if tracer == nil {
		return ctx, nil, ErrNilTracer
	}
	attrs := agentOperationAttributes(op)
	opts = append([]trace.SpanStartOption{trace.WithAttributes(attrs...)}, opts...)
	spanCtx, span := tracer.Start(ctx, agentOperationSpanName(op), opts...)
	return spanCtx, span, nil
}

// StartToolCall starts a GenAI tool execution span.
func StartToolCall(ctx context.Context, tracer trace.Tracer, call ToolCall, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	if tracer == nil {
		return ctx, nil, ErrNilTracer
	}
	attrs := toolCallAttributes(call)
	opts = append([]trace.SpanStartOption{trace.WithAttributes(attrs...)}, opts...)
	spanCtx, span := tracer.Start(ctx, toolCallSpanName(call), opts...)
	return spanCtx, span, nil
}

// AddFallbackEvent records a fallback event on span with bounded fallback attrs.
func AddFallbackEvent(span trace.Span, fallback Fallback) {
	if span == nil {
		return
	}
	span.AddEvent(MetricAgentOTelFallbackEngaged, trace.WithAttributes(fallbackAttributes(fallback)...))
}

// MarkSpanError records err, sets error status, and emits error.type when provided.
func MarkSpanError(span trace.Span, err error, errorType string) {
	if span == nil {
		return
	}
	if errorType != "" {
		span.SetAttributes(attribute.String(AttrErrorType, errorType))
	}
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return
	}
	if errorType != "" {
		span.SetStatus(codes.Error, errorType)
	}
}

// StartModelCall starts a GenAI model-call span with this provider's tracer.
func (p *Providers) StartModelCall(ctx context.Context, call ModelCall, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	if p == nil {
		return ctx, nil, ErrNilTracer
	}
	return startModelCall(ctx, p.Tracer, call, p.openLLMetryCompat, p.payloadCapture, opts...)
}

// StartAgentOperation starts a GenAI agent operation span with this provider's tracer.
func (p *Providers) StartAgentOperation(ctx context.Context, op AgentOperation, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	if p == nil {
		return ctx, nil, ErrNilTracer
	}
	return StartAgentOperation(ctx, p.Tracer, op, opts...)
}

// StartToolCall starts a GenAI tool execution span with this provider's tracer.
func (p *Providers) StartToolCall(ctx context.Context, call ToolCall, opts ...trace.SpanStartOption) (context.Context, trace.Span, error) {
	if p == nil {
		return ctx, nil, ErrNilTracer
	}
	return StartToolCall(ctx, p.Tracer, call, opts...)
}

func modelCallAttributes(call ModelCall) ([]attribute.KeyValue, error) {
	attrs := make([]attribute.KeyValue, 0, 8+len(call.Attributes))
	if call.OperationName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIOperationName, call.OperationName))
	}
	if call.ProviderName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIProviderName, call.ProviderName))
	}
	if call.System != "" {
		attrs = append(attrs, attribute.String(AttrGenAISystem, call.System))
	}
	if call.RequestModel != "" {
		attrs = append(attrs, attribute.String(AttrGenAIRequestModel, call.RequestModel))
	}
	if call.ResponseModel != "" {
		attrs = append(attrs, attribute.String(AttrGenAIResponseModel, call.ResponseModel))
	}
	usageAttrs, err := UsageSpanAttributes(call.Usage)
	if err != nil {
		return nil, err
	}
	attrs = append(attrs, usageAttrs...)
	attrs = append(attrs, call.Attributes...)
	return attrs, nil
}

func agentOperationAttributes(op AgentOperation) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 2+len(op.Attributes))
	if op.OperationName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIOperationName, op.OperationName))
	}
	if op.AgentName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIAgentName, op.AgentName))
	}
	attrs = append(attrs, op.Attributes...)
	return attrs
}

func toolCallAttributes(call ToolCall) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 2+len(call.Attributes))
	attrs = append(attrs, attribute.String(AttrGenAIOperationName, GenAIOperationExecuteTool))
	if call.ToolName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIToolName, call.ToolName))
	}
	attrs = append(attrs, call.Attributes...)
	return attrs
}

func fallbackAttributes(fallback Fallback) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 3+len(fallback.Attributes))
	if fallback.ProviderName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIProviderName, fallback.ProviderName))
	}
	if fallback.FromProvider != "" {
		attrs = append(attrs, attribute.String(AttrAgentOTelProviderFrom, fallback.FromProvider))
	}
	if fallback.ToProvider != "" {
		attrs = append(attrs, attribute.String(AttrAgentOTelProviderTo, fallback.ToProvider))
	}
	attrs = append(attrs, fallback.Attributes...)
	return attrs
}

func modelCallSpanName(call ModelCall) string {
	if call.OperationName == "" {
		return call.RequestModel
	}
	if call.RequestModel == "" {
		return call.OperationName
	}
	return call.OperationName + " " + call.RequestModel
}

func agentOperationSpanName(op AgentOperation) string {
	if op.OperationName == "" {
		return op.AgentName
	}
	if op.AgentName == "" {
		return op.OperationName
	}
	return op.OperationName + " " + op.AgentName
}

func toolCallSpanName(call ToolCall) string {
	if call.ToolName == "" {
		return GenAIOperationExecuteTool
	}
	return GenAIOperationExecuteTool + " " + call.ToolName
}
