package agentotel

const (
	AttrGenAIAgentName             = "gen_ai.agent.name"
	AttrGenAIInputMessages         = "gen_ai.input.messages"
	AttrGenAIOperationName         = "gen_ai.operation.name"
	AttrGenAIOutputMessages        = "gen_ai.output.messages"
	AttrGenAIProviderName          = "gen_ai.provider.name"
	AttrGenAIRequestMaxTokens      = "gen_ai.request.max_tokens"
	AttrGenAIRequestModel          = "gen_ai.request.model"
	AttrGenAIRequestTemperature    = "gen_ai.request.temperature"
	AttrGenAIRequestTopP           = "gen_ai.request.top_p"
	AttrGenAIResponseFinishReasons = "gen_ai.response.finish_reasons"
	AttrGenAIResponseModel         = "gen_ai.response.model"
	AttrGenAISystem                = "gen_ai.system"
	AttrGenAISystemInstructions    = "gen_ai.system_instructions"
	AttrGenAITokenType             = "gen_ai.token.type"
	AttrGenAIToolDefinitions       = "gen_ai.tool.definitions"
	AttrGenAIToolName              = "gen_ai.tool.name"
	AttrGenAIUsageInputTokens      = "gen_ai.usage.input_tokens"
	AttrGenAIUsageOutputTokens     = "gen_ai.usage.output_tokens"
	AttrGenAIUsageTotalTokens      = "gen_ai.usage.total_tokens"
	AttrErrorType                  = "error.type"
)

const (
	EventGenAIClientInferenceOperationDetails = "gen_ai.client.inference.operation.details"
)

const (
	MetricGenAIClientOperationDuration = "gen_ai.client.operation.duration"
	MetricGenAIClientTokenUsage        = "gen_ai.client.token.usage"
)

const (
	GenAIOperationChat            = "chat"
	GenAIOperationCreateAgent     = "create_agent"
	GenAIOperationEmbeddings      = "embeddings"
	GenAIOperationExecuteTool     = "execute_tool"
	GenAIOperationGenerateContent = "generate_content"
	GenAIOperationInvokeAgent     = "invoke_agent"
	GenAIOperationTextCompletion  = "text_completion"
)

const (
	GenAITokenTypeInput  = "input"
	GenAITokenTypeOutput = "output"
)
