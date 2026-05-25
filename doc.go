// Package agentotel provides shared OpenTelemetry helpers for Go agent
// services.
//
// Init builds trace, metric, and log providers from OpenTelemetry OTLP
// options and environment variables. The package also provides GenAI span
// helpers, normalized usage helpers, cardinality-checked model-call metrics,
// optional Datadog and OpenLLMetry compatibility presets, and opt-in redacted
// payload capture.
//
// The initial consumers are github.com/mattsp1290/advisor and
// github.com/mattsp1290/local-symphony.
package agentotel
