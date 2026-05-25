package agentotel

import (
	"encoding/json"

	"go.opentelemetry.io/otel/attribute"
)

const (
	AttrLLMRequestType       = "llm.request.type"
	AttrLLMUsageTotalTokens  = "llm.usage.total_tokens"
	AttrLLMRequestFunctions  = "llm.request.functions"
	AttrTraceloopSpanKind    = "traceloop.span.kind"
	AttrTraceloopWorkflow    = "traceloop.workflow.name"
	AttrTraceloopEntityName  = "traceloop.entity.name"
	AttrTraceloopEntityPath  = "traceloop.entity.path"
	AttrTraceloopEntityVer   = "traceloop.entity.version"
	AttrTraceloopAssocProps  = "traceloop.association.properties"
	LLMRequestTypeChat       = "chat"
	LLMRequestTypeCompletion = "completion"
	LLMRequestTypeUnknown    = "unknown"
)

// TraceloopSpanKind is a bounded Traceloop span-kind compatibility value.
type TraceloopSpanKind string

const (
	TraceloopWorkflow TraceloopSpanKind = "workflow"
	TraceloopTask     TraceloopSpanKind = "task"
	TraceloopAgent    TraceloopSpanKind = "agent"
	TraceloopTool     TraceloopSpanKind = "tool"
	TraceloopSession  TraceloopSpanKind = "session"
	TraceloopUnknown  TraceloopSpanKind = "unknown"
)

// OpenLLMetryCompatOptions controls additive legacy llm.* and traceloop.* attrs.
type OpenLLMetryCompatOptions struct {
	WorkflowName          string
	EntityName            string
	EntityPath            string
	EntityVersion         string
	SpanKind              TraceloopSpanKind
	AssociationProperties map[string]string
}

// WithOpenLLMetryCompat enables additive OpenLLMetry legacy attributes.
func WithOpenLLMetryCompat(options ...OpenLLMetryCompatOptions) Option {
	return func(opts *Options) {
		compat := OpenLLMetryCompatOptions{}
		if opts.OpenLLMetryCompat != nil {
			compat = *opts.OpenLLMetryCompat
			compat.AssociationProperties = cloneStringMap(compat.AssociationProperties)
		}
		for _, opt := range options {
			compat = mergeOpenLLMetryCompat(compat, opt)
		}
		opts.OpenLLMetryCompat = &compat
	}
}

func openLLMetryModelCallAttributes(call ModelCall, compat *OpenLLMetryCompatOptions) []attribute.KeyValue {
	if compat == nil {
		return nil
	}
	attrs := []attribute.KeyValue{
		attribute.String(AttrLLMRequestType, openLLMetryRequestType(call.OperationName)),
	}
	if call.Usage.Available {
		attrs = append(attrs, attribute.Int64(AttrLLMUsageTotalTokens, call.Usage.InputTokens+call.Usage.OutputTokens))
	}
	attrs = append(attrs, openLLMetryTraceloopAttributes(*compat)...)
	return attrs
}

func openLLMetryTraceloopAttributes(compat OpenLLMetryCompatOptions) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 6)
	if compat.SpanKind != "" {
		attrs = append(attrs, attribute.String(AttrTraceloopSpanKind, string(compat.SpanKind)))
	}
	if compat.WorkflowName != "" {
		attrs = append(attrs, attribute.String(AttrTraceloopWorkflow, compat.WorkflowName))
	}
	if compat.EntityName != "" {
		attrs = append(attrs, attribute.String(AttrTraceloopEntityName, compat.EntityName))
	}
	if compat.EntityPath != "" {
		attrs = append(attrs, attribute.String(AttrTraceloopEntityPath, compat.EntityPath))
	}
	if compat.EntityVersion != "" {
		attrs = append(attrs, attribute.String(AttrTraceloopEntityVer, compat.EntityVersion))
	}
	if len(compat.AssociationProperties) > 0 {
		if encoded, err := json.Marshal(compat.AssociationProperties); err == nil {
			attrs = append(attrs, attribute.String(AttrTraceloopAssocProps, string(encoded)))
		}
	}
	return attrs
}

func openLLMetryRequestType(operation string) string {
	switch operation {
	case GenAIOperationChat:
		return LLMRequestTypeChat
	case GenAIOperationTextCompletion:
		return LLMRequestTypeCompletion
	default:
		return LLMRequestTypeUnknown
	}
}

func mergeOpenLLMetryCompat(base, overlay OpenLLMetryCompatOptions) OpenLLMetryCompatOptions {
	if overlay.WorkflowName != "" {
		base.WorkflowName = overlay.WorkflowName
	}
	if overlay.EntityName != "" {
		base.EntityName = overlay.EntityName
	}
	if overlay.EntityPath != "" {
		base.EntityPath = overlay.EntityPath
	}
	if overlay.EntityVersion != "" {
		base.EntityVersion = overlay.EntityVersion
	}
	if overlay.SpanKind != "" {
		base.SpanKind = overlay.SpanKind
	}
	if len(overlay.AssociationProperties) > 0 {
		if base.AssociationProperties == nil {
			base.AssociationProperties = make(map[string]string, len(overlay.AssociationProperties))
		}
		for key, value := range overlay.AssociationProperties {
			base.AssociationProperties[key] = value
		}
	}
	return base
}
