package agentotel

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

const AttrAgentOTelUsageAvailable = "agent_otel.usage.available"

var ErrNegativeUsage = errors.New("agentotel: usage token counts must be non-negative")

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	Available    bool
}

type UsageMetricPoint struct {
	TokenType string
	Tokens    int64
}

func (u Usage) Validate() error {
	if u.InputTokens < 0 {
		return fmt.Errorf("%w: input_tokens=%d", ErrNegativeUsage, u.InputTokens)
	}
	if u.OutputTokens < 0 {
		return fmt.Errorf("%w: output_tokens=%d", ErrNegativeUsage, u.OutputTokens)
	}
	return nil
}

func UsageSpanAttributes(usage Usage) ([]attribute.KeyValue, error) {
	if err := usage.Validate(); err != nil {
		return nil, err
	}

	attrs := []attribute.KeyValue{
		attribute.Bool(AttrAgentOTelUsageAvailable, usage.Available),
	}
	if !usage.Available {
		return attrs, nil
	}

	return append(attrs,
		attribute.Int64(AttrGenAIUsageInputTokens, usage.InputTokens),
		attribute.Int64(AttrGenAIUsageOutputTokens, usage.OutputTokens),
	), nil
}

func UsageMetricPoints(usage Usage) ([]UsageMetricPoint, error) {
	if err := usage.Validate(); err != nil {
		return nil, err
	}
	if !usage.Available {
		return nil, nil
	}
	return []UsageMetricPoint{
		{TokenType: GenAITokenTypeInput, Tokens: usage.InputTokens},
		{TokenType: GenAITokenTypeOutput, Tokens: usage.OutputTokens},
	}, nil
}
