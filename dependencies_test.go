package agentotel

import (
	_ "github.com/stretchr/testify/require"
	_ "go.opentelemetry.io/contrib/bridges/otelslog"
	_ "go.opentelemetry.io/otel"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	_ "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	_ "go.opentelemetry.io/otel/log"
	_ "go.opentelemetry.io/otel/metric"
	_ "go.opentelemetry.io/otel/sdk"
	_ "go.opentelemetry.io/otel/sdk/log"
	_ "go.opentelemetry.io/otel/sdk/log/logtest"
	_ "go.opentelemetry.io/otel/sdk/metric"
	_ "go.opentelemetry.io/otel/semconv/v1.40.0"
	_ "go.opentelemetry.io/otel/trace"
)
