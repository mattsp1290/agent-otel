package agentotel

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestAllowedKeysAcceptBuiltInMetrics(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		MetricGenAIClientOperationDuration: {
			AttrGenAIProviderName,
			AttrGenAIRequestModel,
			AttrGenAIOperationName,
			AttrErrorType,
		},
		MetricGenAIClientTokenUsage: {
			AttrGenAIProviderName,
			AttrGenAIRequestModel,
			AttrGenAIOperationName,
			AttrGenAITokenType,
		},
		MetricAgentOTelProviderErrors: {
			AttrGenAIProviderName,
			AttrErrorType,
		},
		MetricAgentOTelFallbackEngaged: {
			AttrGenAIProviderName,
			AttrAgentOTelProviderFrom,
			AttrAgentOTelProviderTo,
		},
	}

	for metricName, keys := range tests {
		t.Run(metricName, func(t *testing.T) {
			t.Parallel()
			got, ok := AllowedKeys(metricName)
			require.True(t, ok)
			require.ElementsMatch(t, keys, got)
			require.NoError(t, CheckAllowedLabels(metricName, keys...))
		})
	}
}

func TestAllowedAndProhibitedAccessorsReturnCopies(t *testing.T) {
	t.Parallel()

	allowed, ok := AllowedKeys(MetricGenAIClientOperationDuration)
	require.True(t, ok)
	allowed[0] = "mutated"
	allowedAgain, ok := AllowedKeys(MetricGenAIClientOperationDuration)
	require.True(t, ok)
	require.NotContains(t, allowedAgain, "mutated")

	prohibited := ProhibitedKeys()
	prohibited[0] = "mutated"
	require.NotContains(t, ProhibitedKeys(), "mutated")

	prefixes := ProhibitedPrefixes()
	prefixes[0] = "mutated"
	require.NotContains(t, ProhibitedPrefixes(), "mutated")

	specs := BuiltInMetricSpecs()
	specs[0].AllowedKeys[0] = "mutated"
	specsAgain := BuiltInMetricSpecs()
	require.NotContains(t, specsAgain[0].AllowedKeys, "mutated")
}

func TestCheckAllowedLabelsRejectsProhibitedAndUnknown(t *testing.T) {
	t.Parallel()

	for _, metricName := range []string{
		MetricGenAIClientOperationDuration,
		MetricGenAIClientTokenUsage,
		MetricAgentOTelProviderErrors,
		MetricAgentOTelFallbackEngaged,
	} {
		t.Run(metricName, func(t *testing.T) {
			t.Parallel()
			require.ErrorContains(t, CheckAllowedLabels(metricName, "issue.id"), ViolationReasonProhibited)
			require.ErrorContains(t, CheckAllowedLabels(metricName, "tool.input/raw"), ViolationReasonProhibited)
			require.ErrorContains(t, CheckAllowedLabels(metricName, AttrGenAIInputMessages), ViolationReasonProhibited)
			require.ErrorContains(t, CheckAllowedLabels(metricName, "user.email"), ViolationReasonUnknownKey)
		})
	}

	require.ErrorContains(t, CheckAllowedLabels("unknown.metric", AttrGenAIProviderName), ViolationReasonUnknownMetric)
}

func TestFilterLogAndDropKeepsAllowedAndDropsBadLabels(t *testing.T) {
	t.Parallel()

	var logs []CardinalityViolation
	v := NewCardinalityValidator(WithViolationLogger(func(violation CardinalityViolation) {
		logs = append(logs, violation)
	}))
	attrs := []attribute.KeyValue{
		attribute.String(AttrGenAIProviderName, "openai"),
		attribute.String(AttrGenAIRequestModel, "gpt-4o"),
		attribute.String("session.id", "s-1"),
		attribute.String("user.email", "person@example.test"),
	}

	filtered, record, err := v.Filter(MetricGenAIClientOperationDuration, attrs)
	require.NoError(t, err)
	require.True(t, record)
	require.Equal(t, []attribute.KeyValue{
		attribute.String(AttrGenAIProviderName, "openai"),
		attribute.String(AttrGenAIRequestModel, "gpt-4o"),
	}, filtered)
	require.Equal(t, []CardinalityViolation{
		{
			Metric: MetricGenAIClientOperationDuration,
			Key:    "session.id",
			Reason: ViolationReasonProhibited,
			Action: ViolationActionDroppedLabel,
		},
		{
			Metric: MetricGenAIClientOperationDuration,
			Key:    "user.email",
			Reason: ViolationReasonUnknownKey,
			Action: ViolationActionDroppedLabel,
		},
	}, logs)

	_, _, err = v.Filter(MetricGenAIClientOperationDuration, attrs)
	require.NoError(t, err)
	require.Len(t, logs, 2, "repeat violations should log once per metric/key/reason/action")
}

func TestFilterUnknownMetricDropsMeasurement(t *testing.T) {
	t.Parallel()

	var logs []CardinalityViolation
	v := NewCardinalityValidator(WithViolationLogger(func(violation CardinalityViolation) {
		logs = append(logs, violation)
	}))

	filtered, record, err := v.Filter("unknown.metric", []attribute.KeyValue{
		attribute.String(AttrGenAIProviderName, "openai"),
	})
	require.ErrorContains(t, err, ViolationReasonUnknownMetric)
	require.False(t, record)
	require.Nil(t, filtered)
	require.Equal(t, []CardinalityViolation{
		{
			Metric: "unknown.metric",
			Reason: ViolationReasonUnknownMetric,
			Action: ViolationActionDroppedMeasurement,
		},
	}, logs)
}

func TestStrictModeSurfacesViolationWithoutExportingBadLabel(t *testing.T) {
	t.Parallel()

	v := NewCardinalityValidator(WithCardinalityMode(CardinalityStrict))
	filtered, record, err := v.Filter(MetricGenAIClientOperationDuration, []attribute.KeyValue{
		attribute.String(AttrGenAIProviderName, "openai"),
		attribute.String("thread.id", "t-1"),
	})
	require.ErrorContains(t, err, ViolationReasonProhibited)
	require.True(t, record)
	require.Equal(t, []attribute.KeyValue{
		attribute.String(AttrGenAIProviderName, "openai"),
	}, filtered)
}

func TestDisabledModeBypassesFilteringWithNoAllocations(t *testing.T) {
	v := NewCardinalityValidator(WithCardinalityMode(CardinalityDisabled))
	attrs := []attribute.KeyValue{
		attribute.String(AttrGenAIProviderName, "openai"),
		attribute.String("thread.id", "t-1"),
	}

	filtered, record, err := v.Filter(MetricGenAIClientOperationDuration, attrs)
	require.NoError(t, err)
	require.True(t, record)
	require.Equal(t, attrs, filtered)
	require.Same(t, &attrs[0], &filtered[0])

	allocs := testing.AllocsPerRun(1000, func() {
		got, keep, filterErr := v.Filter(MetricGenAIClientOperationDuration, attrs)
		if !keep || filterErr != nil || len(got) != len(attrs) {
			t.Fatalf("disabled filter = (%v, %v, %v), want passthrough", got, keep, filterErr)
		}
	})
	require.Zero(t, allocs)
}
