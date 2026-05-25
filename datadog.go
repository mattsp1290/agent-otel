package agentotel

const (
	// DatadogHeaderAPIKey is the OTLP header Datadog uses for API keys.
	DatadogHeaderAPIKey = "dd-api-key"
	// DatadogHeaderOTLPSource identifies Datadog LLM Observability OTLP traffic.
	DatadogHeaderOTLPSource = "dd-otlp-source"
	// DatadogOTLPSourceLLMObs is the Datadog LLM Observability OTLP source value.
	DatadogOTLPSourceLLMObs = "llmobs"
	// DatadogSemconvStabilityOptIn is the recommended semconv opt-in value for latest GenAI conventions.
	DatadogSemconvStabilityOptIn = "gen_ai_latest_experimental"
	// EnvDatadogAPIKey is the Datadog API key environment variable.
	EnvDatadogAPIKey = "DD_API_KEY"
)

func datadogTraceDefaults(preset *DatadogPreset, lookup lookupEnv) (endpoint string, headers map[string]string, protocol string) {
	if preset == nil || !preset.Enabled {
		return "", nil, ""
	}

	headers = cloneStringMap(preset.Headers)
	apiKey := preset.APIKey
	if apiKey == "" {
		apiKey, _ = lookup(EnvDatadogAPIKey)
	}
	if apiKey != "" {
		if headers == nil {
			headers = make(map[string]string, 2)
		}
		headers[DatadogHeaderAPIKey] = apiKey
	}
	if headers == nil {
		headers = make(map[string]string, 1)
	}
	headers[DatadogHeaderOTLPSource] = DatadogOTLPSourceLLMObs

	return preset.TraceEndpoint, headers, ProtocolHTTPProtobuf
}
