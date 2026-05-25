package agentotel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenAIAttributeLiterals(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"AttrGenAIAgentName":             AttrGenAIAgentName,
		"AttrGenAIInputMessages":         AttrGenAIInputMessages,
		"AttrGenAIOperationName":         AttrGenAIOperationName,
		"AttrGenAIOutputMessages":        AttrGenAIOutputMessages,
		"AttrGenAIProviderName":          AttrGenAIProviderName,
		"AttrGenAIRequestMaxTokens":      AttrGenAIRequestMaxTokens,
		"AttrGenAIRequestModel":          AttrGenAIRequestModel,
		"AttrGenAIRequestTemperature":    AttrGenAIRequestTemperature,
		"AttrGenAIRequestTopP":           AttrGenAIRequestTopP,
		"AttrGenAIResponseFinishReasons": AttrGenAIResponseFinishReasons,
		"AttrGenAIResponseModel":         AttrGenAIResponseModel,
		"AttrGenAISystem":                AttrGenAISystem,
		"AttrGenAISystemInstructions":    AttrGenAISystemInstructions,
		"AttrGenAITokenType":             AttrGenAITokenType,
		"AttrGenAIToolDefinitions":       AttrGenAIToolDefinitions,
		"AttrGenAIToolName":              AttrGenAIToolName,
		"AttrGenAIUsageInputTokens":      AttrGenAIUsageInputTokens,
		"AttrGenAIUsageOutputTokens":     AttrGenAIUsageOutputTokens,
		"AttrGenAIUsageTotalTokens":      AttrGenAIUsageTotalTokens,
		"AttrErrorType":                  AttrErrorType,
	}

	want := map[string]string{
		"AttrGenAIAgentName":             "gen_ai.agent.name",
		"AttrGenAIInputMessages":         "gen_ai.input.messages",
		"AttrGenAIOperationName":         "gen_ai.operation.name",
		"AttrGenAIOutputMessages":        "gen_ai.output.messages",
		"AttrGenAIProviderName":          "gen_ai.provider.name",
		"AttrGenAIRequestMaxTokens":      "gen_ai.request.max_tokens",
		"AttrGenAIRequestModel":          "gen_ai.request.model",
		"AttrGenAIRequestTemperature":    "gen_ai.request.temperature",
		"AttrGenAIRequestTopP":           "gen_ai.request.top_p",
		"AttrGenAIResponseFinishReasons": "gen_ai.response.finish_reasons",
		"AttrGenAIResponseModel":         "gen_ai.response.model",
		"AttrGenAISystem":                "gen_ai.system",
		"AttrGenAISystemInstructions":    "gen_ai.system_instructions",
		"AttrGenAITokenType":             "gen_ai.token.type",
		"AttrGenAIToolDefinitions":       "gen_ai.tool.definitions",
		"AttrGenAIToolName":              "gen_ai.tool.name",
		"AttrGenAIUsageInputTokens":      "gen_ai.usage.input_tokens",
		"AttrGenAIUsageOutputTokens":     "gen_ai.usage.output_tokens",
		"AttrGenAIUsageTotalTokens":      "gen_ai.usage.total_tokens",
		"AttrErrorType":                  "error.type",
	}

	require.Equal(t, want, tests)
}

func TestGenAIEventAndMetricLiterals(t *testing.T) {
	t.Parallel()

	require.Equal(t, "gen_ai.client.inference.operation.details", EventGenAIClientInferenceOperationDetails)
	require.Equal(t, "gen_ai.client.operation.duration", MetricGenAIClientOperationDuration)
	require.Equal(t, "gen_ai.client.token.usage", MetricGenAIClientTokenUsage)
}

func TestGenAIValueLiterals(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"GenAIOperationChat":            GenAIOperationChat,
		"GenAIOperationCreateAgent":     GenAIOperationCreateAgent,
		"GenAIOperationEmbeddings":      GenAIOperationEmbeddings,
		"GenAIOperationExecuteTool":     GenAIOperationExecuteTool,
		"GenAIOperationGenerateContent": GenAIOperationGenerateContent,
		"GenAIOperationInvokeAgent":     GenAIOperationInvokeAgent,
		"GenAIOperationTextCompletion":  GenAIOperationTextCompletion,
		"GenAITokenTypeInput":           GenAITokenTypeInput,
		"GenAITokenTypeOutput":          GenAITokenTypeOutput,
	}

	want := map[string]string{
		"GenAIOperationChat":            "chat",
		"GenAIOperationCreateAgent":     "create_agent",
		"GenAIOperationEmbeddings":      "embeddings",
		"GenAIOperationExecuteTool":     "execute_tool",
		"GenAIOperationGenerateContent": "generate_content",
		"GenAIOperationInvokeAgent":     "invoke_agent",
		"GenAIOperationTextCompletion":  "text_completion",
		"GenAITokenTypeInput":           "input",
		"GenAITokenTypeOutput":          "output",
	}

	require.Equal(t, want, tests)
}
