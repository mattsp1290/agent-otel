// Package agentotel provides shared OpenTelemetry helpers for Go agent
// services.
//
// The initial consumers are github.com/mattsp1290/advisor and
// github.com/mattsp1290/local-symphony. This package intentionally starts as
// a behavior-free skeleton; implementation beads add bootstrap, GenAI
// instrumentation, redaction, and metric-cardinality enforcement in small
// steps.
package agentotel
