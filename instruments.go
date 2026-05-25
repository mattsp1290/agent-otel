package agentotel

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ModelMetricLabels are bounded GenAI labels for model-call metrics.
type ModelMetricLabels struct {
	OperationName string
	ProviderName  string
	RequestModel  string
	ErrorType     string
}

// ProviderErrorLabels are bounded labels for provider error metrics.
type ProviderErrorLabels struct {
	ProviderName string
	ErrorType    string
}

// RecordModelLatency records a GenAI model-call duration in seconds.
func (i *Instruments) RecordModelLatency(ctx context.Context, seconds float64, labels ModelMetricLabels) error {
	if i == nil {
		return nil
	}
	attrs, ok, err := i.filterMetricAttrs(MetricGenAIClientOperationDuration, modelLatencyAttrs(labels))
	if err != nil || !ok {
		return err
	}
	i.ModelLatency.Record(ctx, seconds, metric.WithAttributes(attrs...))
	return nil
}

// RecordUsage records input and output token usage when usage is available.
func (i *Instruments) RecordUsage(ctx context.Context, usage Usage, labels ModelMetricLabels) error {
	points, err := UsageMetricPoints(usage)
	if err != nil {
		return err
	}
	for _, point := range points {
		if point.TokenType == GenAITokenTypeInput {
			if err := i.RecordUsageInputTokens(ctx, point.Tokens, labels); err != nil {
				return err
			}
			continue
		}
		if err := i.RecordUsageOutputTokens(ctx, point.Tokens, labels); err != nil {
			return err
		}
	}
	return nil
}

// RecordUsageInputTokens records a GenAI input-token data point.
func (i *Instruments) RecordUsageInputTokens(ctx context.Context, tokens int64, labels ModelMetricLabels) error {
	return i.recordUsageTokens(ctx, tokens, labels, GenAITokenTypeInput)
}

// RecordUsageOutputTokens records a GenAI output-token data point.
func (i *Instruments) RecordUsageOutputTokens(ctx context.Context, tokens int64, labels ModelMetricLabels) error {
	return i.recordUsageTokens(ctx, tokens, labels, GenAITokenTypeOutput)
}

// RecordProviderError increments the bounded provider error counter.
func (i *Instruments) RecordProviderError(ctx context.Context, labels ProviderErrorLabels) error {
	if i == nil {
		return nil
	}
	attrs, ok, err := i.filterMetricAttrs(MetricAgentOTelProviderErrors, providerErrorAttrs(labels))
	if err != nil || !ok {
		return err
	}
	i.ErrorsByProvider.Add(ctx, 1, metric.WithAttributes(attrs...))
	return nil
}

// RecordFallbackEngaged increments the bounded provider fallback counter.
func (i *Instruments) RecordFallbackEngaged(ctx context.Context, fallback Fallback) error {
	if i == nil {
		return nil
	}
	attrs, ok, err := i.filterMetricAttrs(MetricAgentOTelFallbackEngaged, fallbackAttributes(fallback))
	if err != nil || !ok {
		return err
	}
	i.FallbackEngaged.Add(ctx, 1, metric.WithAttributes(attrs...))
	return nil
}

func (i *Instruments) recordUsageTokens(ctx context.Context, tokens int64, labels ModelMetricLabels, tokenType string) error {
	if i == nil {
		return nil
	}
	if tokens < 0 {
		return fmt.Errorf("%w: %s_tokens=%d", ErrNegativeUsage, tokenType, tokens)
	}
	attrs, ok, err := i.filterMetricAttrs(MetricGenAIClientTokenUsage, tokenUsageAttrs(labels, tokenType))
	if err != nil || !ok {
		return err
	}
	i.tokenUsageRecorder.Record(ctx, tokens, metric.WithAttributes(attrs...))
	return nil
}

func (i *Instruments) filterMetricAttrs(metricName string, attrs []attribute.KeyValue) ([]attribute.KeyValue, bool, error) {
	if i == nil || i.cardinality == nil {
		return attrs, true, nil
	}
	return i.cardinality.Filter(metricName, attrs)
}

func modelLatencyAttrs(labels ModelMetricLabels) []attribute.KeyValue {
	attrs := modelBaseAttrs(labels)
	if labels.ErrorType != "" {
		attrs = append(attrs, attribute.String(AttrErrorType, labels.ErrorType))
	}
	return attrs
}

func tokenUsageAttrs(labels ModelMetricLabels, tokenType string) []attribute.KeyValue {
	attrs := modelBaseAttrs(labels)
	attrs = append(attrs, attribute.String(AttrGenAITokenType, tokenType))
	return attrs
}

func modelBaseAttrs(labels ModelMetricLabels) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 4)
	if labels.OperationName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIOperationName, labels.OperationName))
	}
	if labels.ProviderName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIProviderName, labels.ProviderName))
	}
	if labels.RequestModel != "" {
		attrs = append(attrs, attribute.String(AttrGenAIRequestModel, labels.RequestModel))
	}
	return attrs
}

func providerErrorAttrs(labels ProviderErrorLabels) []attribute.KeyValue {
	attrs := make([]attribute.KeyValue, 0, 2)
	if labels.ProviderName != "" {
		attrs = append(attrs, attribute.String(AttrGenAIProviderName, labels.ProviderName))
	}
	if labels.ErrorType != "" {
		attrs = append(attrs, attribute.String(AttrErrorType, labels.ErrorType))
	}
	return attrs
}
