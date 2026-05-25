package agentotel

import (
	"fmt"
	"slices"

	"go.opentelemetry.io/otel/attribute"
)

const (
	AttrAgentOTelProviderFrom = "agent_otel.provider.from"
	AttrAgentOTelProviderTo   = "agent_otel.provider.to"

	MetricAgentOTelProviderErrors     = "agent_otel.provider.errors"
	MetricAgentOTelFallbackEngaged    = "agent_otel.fallback.engaged"
	ViolationActionDroppedLabel       = "dropped_label"
	ViolationActionDroppedMeasurement = "dropped_measurement"
	ViolationReasonProhibited         = "prohibited"
	ViolationReasonUnknownKey         = "unknown_key"
	ViolationReasonUnknownMetric      = "unknown_metric"
	ViolationReasonInvalidValue       = "invalid_value"
)

type MetricSpec struct {
	Name        string
	Unit        string
	Description string
	AllowedKeys []string
}

type CardinalityMode int

const (
	CardinalityLogAndDrop CardinalityMode = iota
	CardinalityStrict
	CardinalityDisabled
)

type CardinalityViolation struct {
	Metric string
	Key    string
	Reason string
	Action string
}

type ViolationLogger func(CardinalityViolation)

type CardinalityValidator struct {
	mode   CardinalityMode
	logger ViolationLogger
	seen   map[violationKey]struct{}
}

type CardinalityOption func(*CardinalityValidator)

type violationKey struct {
	metric string
	key    string
	reason string
	action string
}

var builtInMetricSpecs = []MetricSpec{
	{
		Name:        MetricGenAIClientOperationDuration,
		Unit:        "s",
		Description: "Duration of a GenAI client operation.",
		AllowedKeys: []string{
			AttrGenAIProviderName,
			AttrGenAIRequestModel,
			AttrGenAIOperationName,
			AttrErrorType,
		},
	},
	{
		Name:        MetricGenAIClientTokenUsage,
		Unit:        "{token}",
		Description: "Token usage by token type for a GenAI client operation.",
		AllowedKeys: []string{
			AttrGenAIProviderName,
			AttrGenAIRequestModel,
			AttrGenAIOperationName,
			AttrGenAITokenType,
		},
	},
	{
		Name:        MetricAgentOTelProviderErrors,
		Unit:        "{error}",
		Description: "Provider errors by bounded provider and error type.",
		AllowedKeys: []string{
			AttrGenAIProviderName,
			AttrErrorType,
		},
	},
	{
		Name:        MetricAgentOTelFallbackEngaged,
		Unit:        "{event}",
		Description: "Provider fallback activations by bounded provider route.",
		AllowedKeys: []string{
			AttrGenAIProviderName,
			AttrAgentOTelProviderFrom,
			AttrAgentOTelProviderTo,
		},
	},
}

var prohibitedMetricKeys = []string{
	"issue.id",
	"session.id",
	"thread.id",
	"turn.id",
	"run_attempt.id",
	AttrGenAIInputMessages,
	AttrGenAIOutputMessages,
	AttrGenAISystemInstructions,
	AttrGenAIToolDefinitions,
}

var prohibitedMetricPrefixes = []string{
	"tool.input/",
}

func WithCardinalityMode(mode CardinalityMode) CardinalityOption {
	return func(v *CardinalityValidator) {
		v.mode = mode
	}
}

func WithViolationLogger(logger ViolationLogger) CardinalityOption {
	return func(v *CardinalityValidator) {
		v.logger = logger
	}
}

func NewCardinalityValidator(opts ...CardinalityOption) *CardinalityValidator {
	v := &CardinalityValidator{
		mode: CardinalityLogAndDrop,
	}
	for _, opt := range opts {
		opt(v)
	}
	if v.mode != CardinalityDisabled && v.logger != nil {
		v.seen = make(map[violationKey]struct{})
	}
	return v
}

func BuiltInMetricSpecs() []MetricSpec {
	out := make([]MetricSpec, len(builtInMetricSpecs))
	for i, spec := range builtInMetricSpecs {
		out[i] = spec
		out[i].AllowedKeys = slices.Clone(spec.AllowedKeys)
	}
	return out
}

func AllowedKeys(metricName string) ([]string, bool) {
	spec, ok := metricSpec(metricName)
	if !ok {
		return nil, false
	}
	return slices.Clone(spec.AllowedKeys), true
}

func ProhibitedKeys() []string {
	return slices.Clone(prohibitedMetricKeys)
}

func ProhibitedPrefixes() []string {
	return slices.Clone(prohibitedMetricPrefixes)
}

func IsProhibitedKey(key string) bool {
	for _, prohibited := range prohibitedMetricKeys {
		if key == prohibited {
			return true
		}
	}
	for _, prefix := range prohibitedMetricPrefixes {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

func CheckAllowedLabels(metricName string, keys ...string) error {
	spec, ok := metricSpec(metricName)
	if !ok {
		return cardinalityError(metricName, "", ViolationReasonUnknownMetric)
	}
	for _, key := range keys {
		if IsProhibitedKey(key) {
			return cardinalityError(metricName, key, ViolationReasonProhibited)
		}
		if !contains(spec.AllowedKeys, key) {
			return cardinalityError(metricName, key, ViolationReasonUnknownKey)
		}
	}
	return nil
}

func (v *CardinalityValidator) Filter(metricName string, attrs []attribute.KeyValue) ([]attribute.KeyValue, bool, error) {
	if v == nil || v.mode == CardinalityDisabled {
		return attrs, true, nil
	}

	spec, ok := metricSpec(metricName)
	if !ok {
		err := cardinalityError(metricName, "", ViolationReasonUnknownMetric)
		v.log(CardinalityViolation{
			Metric: metricName,
			Reason: ViolationReasonUnknownMetric,
			Action: ViolationActionDroppedMeasurement,
		})
		return nil, false, err
	}

	var firstErr error
	var filtered []attribute.KeyValue
	for _, attr := range attrs {
		key := string(attr.Key)
		reason := ""
		switch {
		case IsProhibitedKey(key):
			reason = ViolationReasonProhibited
		case !contains(spec.AllowedKeys, key):
			reason = ViolationReasonUnknownKey
		}
		if reason != "" {
			err := cardinalityError(metricName, key, reason)
			if firstErr == nil {
				firstErr = err
			}
			v.log(CardinalityViolation{
				Metric: metricName,
				Key:    key,
				Reason: reason,
				Action: ViolationActionDroppedLabel,
			})
			continue
		}
		filtered = append(filtered, attr)
	}

	if v.mode == CardinalityStrict && firstErr != nil {
		return filtered, true, firstErr
	}
	return filtered, true, nil
}

func (v *CardinalityValidator) log(violation CardinalityViolation) {
	if v.logger == nil {
		return
	}
	key := violationKey{
		metric: violation.Metric,
		key:    violation.Key,
		reason: violation.Reason,
		action: violation.Action,
	}
	if _, ok := v.seen[key]; ok {
		return
	}
	v.seen[key] = struct{}{}
	v.logger(violation)
}

func metricSpec(name string) (MetricSpec, bool) {
	for _, spec := range builtInMetricSpecs {
		if spec.Name == name {
			return spec, true
		}
	}
	return MetricSpec{}, false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func cardinalityError(metricName, key, reason string) error {
	if key == "" {
		return fmt.Errorf("cardinality: metric %q rejected: %s", metricName, reason)
	}
	return fmt.Errorf("cardinality: metric %q label %q rejected: %s", metricName, key, reason)
}
