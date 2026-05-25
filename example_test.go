package agentotel_test

import (
	"context"
	"time"

	agentotel "github.com/mattsp1290/agent-otel"
)

func ExampleInit() {
	ctx := context.Background()
	providers, shutdown, err := agentotel.Init(ctx, agentotel.Options{
		Enabled:           false,
		ServiceName:       "example-agent",
		ServiceVersion:    "1.2.3",
		Environment:       "development",
		SkipGlobalInstall: true,
	})
	if err != nil {
		panic(err)
	}
	defer shutdown.Shutdown(context.Background())

	ctx, span, err := agentotel.StartModelCall(ctx, providers.Tracer, agentotel.ModelCall{
		OperationName: agentotel.GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
		Usage: agentotel.Usage{
			InputTokens:  120,
			OutputTokens: 80,
			Available:    true,
		},
	})
	if err != nil {
		panic(err)
	}
	defer span.End()

	_ = providers.Instruments.RecordModelLatency(ctx, 420*time.Millisecond.Seconds(), agentotel.ModelMetricLabels{
		OperationName: agentotel.GenAIOperationChat,
		ProviderName:  "openai",
		RequestModel:  "gpt-4o",
	})
}

func ExampleWithDatadogPreset() {
	_, shutdown, err := agentotel.Init(context.Background(), agentotel.ApplyOptions(agentotel.Options{
		Enabled:           false,
		ServiceName:       "example-agent",
		SkipGlobalInstall: true,
	}, agentotel.WithDatadogPreset(
		agentotel.WithDatadogAPIKey("example"),
		agentotel.WithDatadogTraceEndpoint("https://otlp.datadoghq.com/v1/traces"),
	)))
	if err != nil {
		panic(err)
	}
	defer shutdown.Shutdown(context.Background())
}

func ExampleWithDevSink() {
	_, shutdown, err := agentotel.Init(context.Background(), agentotel.ApplyOptions(agentotel.Options{
		Enabled:           false,
		ServiceName:       "example-agent",
		SkipGlobalInstall: true,
	}, agentotel.WithDevSink("localhost:4317")))
	if err != nil {
		panic(err)
	}
	defer shutdown.Shutdown(context.Background())
}
