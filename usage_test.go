package agentotel

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
)

func TestUsageAvailableEmitsSpanAttrsAndMetricPoints(t *testing.T) {
	t.Parallel()

	usage := Usage{InputTokens: 100, OutputTokens: 180, Available: true}

	attrs, err := UsageSpanAttributes(usage)
	require.NoError(t, err)
	require.Equal(t, []attribute.KeyValue{
		attribute.Bool(AttrAgentOTelUsageAvailable, true),
		attribute.Int64(AttrGenAIUsageInputTokens, 100),
		attribute.Int64(AttrGenAIUsageOutputTokens, 180),
	}, attrs)

	points, err := UsageMetricPoints(usage)
	require.NoError(t, err)
	require.Equal(t, []UsageMetricPoint{
		{TokenType: GenAITokenTypeInput, Tokens: 100},
		{TokenType: GenAITokenTypeOutput, Tokens: 180},
	}, points)
}

func TestUsageUnavailableEmitsOnlyAvailabilityMarker(t *testing.T) {
	t.Parallel()

	usage := Usage{InputTokens: 100, OutputTokens: 180, Available: false}

	attrs, err := UsageSpanAttributes(usage)
	require.NoError(t, err)
	require.Equal(t, []attribute.KeyValue{
		attribute.Bool(AttrAgentOTelUsageAvailable, false),
	}, attrs)

	points, err := UsageMetricPoints(usage)
	require.NoError(t, err)
	require.Nil(t, points)
}

func TestUsageAvailableZeroTokensAreRealValues(t *testing.T) {
	t.Parallel()

	usage := Usage{Available: true}

	attrs, err := UsageSpanAttributes(usage)
	require.NoError(t, err)
	require.Equal(t, []attribute.KeyValue{
		attribute.Bool(AttrAgentOTelUsageAvailable, true),
		attribute.Int64(AttrGenAIUsageInputTokens, 0),
		attribute.Int64(AttrGenAIUsageOutputTokens, 0),
	}, attrs)

	points, err := UsageMetricPoints(usage)
	require.NoError(t, err)
	require.Equal(t, []UsageMetricPoint{
		{TokenType: GenAITokenTypeInput, Tokens: 0},
		{TokenType: GenAITokenTypeOutput, Tokens: 0},
	}, points)
}

func TestUsageRejectsNegativeCounts(t *testing.T) {
	t.Parallel()

	for _, usage := range []Usage{
		{InputTokens: -1, Available: true},
		{OutputTokens: -1, Available: true},
		{InputTokens: -1, Available: false},
		{OutputTokens: -1, Available: false},
	} {
		err := usage.Validate()
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrNegativeUsage))

		attrs, attrErr := UsageSpanAttributes(usage)
		require.ErrorIs(t, attrErr, ErrNegativeUsage)
		require.Nil(t, attrs)

		points, pointsErr := UsageMetricPoints(usage)
		require.ErrorIs(t, pointsErr, ErrNegativeUsage)
		require.Nil(t, points)
	}
}

func TestRootPackageDoesNotImportEino(t *testing.T) {
	t.Parallel()

	cmd := exec.Command("go", "list", "-deps", ".")
	out, err := cmd.Output()
	require.NoError(t, err)
	require.NotContains(t, strings.Split(string(out), "\n"), "github.com/cloudwego/eino")
}
