#!/bin/bash
# Project: agent-otel
# Generated: 2026-05-25

set -e

if [ ! -d ".beads" ]; then
    bd init
fi

echo "Creating agent-otel project beads..."

# ========================================
# Phase 0: Setup
# ========================================

SETUP_REPO=$(bd create "Initialize Go module github.com/mattsp1290/agent-otel with Go 1.25.5" -p 0 --labels setup --description "Reservation: go.mod, go.sum, README.md, package skeleton. Create the initial library module, pin Go 1.25.5, and add a minimal package layout without product behavior." --acceptance "go test ./... succeeds; go.mod declares module github.com/mattsp1290/agent-otel and Go 1.25.5; README names advisor and local-symphony as first consumers." --silent)

SETUP_DEPS=$(bd create "Pin OpenTelemetry and test dependencies" -p 0 --labels setup --description "Reservation: go.mod, go.sum. Add OTel SDK/exporter/log bridge dependencies, testify, and any minimal test helper dependencies required by the planned implementation." --acceptance "go list -m all shows OTel v1.43-compatible modules, go.opentelemetry.io/otel/semconv/v1.40.0, otelslog bridge, and testify; go test ./... succeeds." --silent)
bd dep add $SETUP_DEPS $SETUP_REPO

SETUP_LINT=$(bd create "Configure golangci-lint v1.61+ from advisor and symphony patterns" -p 1 --labels setup --description "Reservation: .golangci.yml, Makefile or justfile lint targets. Create lint configuration aligned with ~/git/advisor and ~/git/local-symphony where practical, documenting intentional deviations in comments." --acceptance "golangci-lint run ./... passes; the config is present and usable by CI; deviations from source repos are explicitly justified." --silent)
bd dep add $SETUP_LINT $SETUP_DEPS

SETUP_CI=$(bd create "Add GitHub Actions for tests, race, lint, and gated integration suite" -p 1 --labels setup,testing --description "Reservation: .github/workflows/**. Add CI jobs for go test, go test -race, golangci-lint, and integration-tagged lotel tests gated by schedule, manual trigger, or label." --acceptance "Workflow YAML validates by review; jobs include go test ./..., go test -race ./..., golangci-lint run ./..., and an integration job running go test -tags=integration ./... under a non-default gate." --silent)
bd dep add $SETUP_CI $SETUP_LINT

# ========================================
# Phase 0b: Pre-extraction Parallel Work
# ========================================

PRE_SYMPHONY_SEMCONV=$(bd create "Pre-extraction: bump local-symphony OTel semconv and record parity impact" -p 0 --labels pre-extraction,migrate-symphony --description "Reservation: ~/git/local-symphony/internal/obs/**, ~/git/local-symphony/go.mod, ~/git/local-symphony/go.sum, related tests. Perform sibling plan §3a semconv/dependency prep and document any telemetry output changes before extraction." --acceptance "In ~/git/local-symphony, go test ./... passes; semconv bump is complete or explicitly documented as blocked; a short review artifact records old/new telemetry impacts." --silent)

PRE_ADVISOR_ADR=$(bd create "Pre-extraction: supersede advisor ADR-0001 telemetry compatibility clauses" -p 0 --labels pre-extraction,migrate-advisor,docs --description "Reservation: ~/git/advisor/docs/adr/**. Add the sibling plan §3b ADR that supersedes ADR-0001 service.version literal and byte-equal MCP replay constraints for this lockstep migration." --acceptance "A new advisor ADR exists, references ADR-0001 clauses being superseded, states the one-time corpus regeneration approach, and passes repository documentation checks if present." --silent)

PRE_ENV_RECONCILE=$(bd create "Pre-extraction: reconcile symphony env-var precedence with OTel per-signal precedence" -p 0 --labels pre-extraction,core,migrate-symphony,docs --description "Reservation: ~/git/local-symphony/docs/otel-env-vars.md and related env resolution tests. Capture the required flip from symphony's current general-wins behavior to OTel-spec per-signal-wins behavior before shared implementation work depends on it." --acceptance "A concrete source-to-target env precedence note exists; any affected local-symphony tests are updated or marked for migration; go test for touched packages passes." --silent)

# ========================================
# Phase 1: Blocker Design Docs
# ========================================

DOC_BOOTSTRAP=$(bd create "Design docs/bootstrap-design.md for bootstrap, exporters, env resolution, and shutdown" -p 0 --labels docs,core,datadog,lotel-devsink --description "Reservation: docs/bootstrap-design.md. Specify per-signal endpoint/header/insecure/protocol resolution, gRPC vs HTTP URL handling, Datadog header injection, dev-sink SDK composition, fail-open rules, and ForceFlush/Shutdown order." --acceptance "docs/bootstrap-design.md exists; it cites current OTel behavior checked at implementation time; it contains decision tables and concrete tests required for env, exporter, and shutdown behavior." --silent)

DOC_GENAI=$(bd create "Design docs/genai-mapping.md from current OTel GenAI semantic conventions" -p 0 --labels docs,genai-semconv,instruments,testing --description "Reservation: docs/genai-mapping.md. Verify official OTel GenAI span, event, metric, attribute, stability, and OTEL_SEMCONV_STABILITY_OPT_IN behavior; decide local constants and token metric shape." --acceptance "docs/genai-mapping.md names exact span/event/metric/attribute strings, stability status, opt-in behavior, local constant decisions, and serialized OTLP assertions required by tests." --silent)

DOC_MIGRATION=$(bd create "Design docs/telemetry-migration-map.md for advisor and symphony cutovers" -p 0 --labels docs,migrate-advisor,migrate-symphony,pre-extraction --description "Reservation: docs/telemetry-migration-map.md plus read-only inspection of ~/git/advisor and ~/git/local-symphony telemetry call sites. Map old spans, metrics, attrs, and tests to agent-otel helpers and GenAI output." --acceptance "Migration map includes advisor provider/model/token/latency/entrypoint/error attrs and symphony model/tool/fallback/Ollama attrs; each row lists target API, retained attrs, dropped attrs, and required tests." --silent)

DOC_DATADOG=$(bd create "Design docs/datadog.md for Datadog LLM Observability mapping" -p 0 --labels docs,datadog,genai-semconv --description "Reservation: docs/datadog.md. Verify current Datadog OTel GenAI LLM Observability expectations, required headers, preset behavior, semconv opt-in, sample values, and on-wire tests." --acceptance "docs/datadog.md lists verified Datadog attribute expectations for gen_ai.request.model, gen_ai.usage.input_tokens, gen_ai.usage.output_tokens, gen_ai.provider.name, and gen_ai.operation.name; preset defaults and tests are explicit." --silent)

DOC_OPENLLMETRY=$(bd create "Design docs/openllmetry.md for native and legacy compatibility attributes" -p 0 --labels docs,openllmetry,genai-semconv --description "Reservation: docs/openllmetry.md. Document OTel-native attributes, optional legacy llm.* and traceloop.* emissions, overlap handling, and compatibility test cases." --acceptance "docs/openllmetry.md includes a mapping table from GenAI attributes to OpenLLMetry legacy attributes, states default-off compatibility behavior, and names exact tests to assert both modes." --silent)

DOC_CARDINALITY=$(bd create "Design docs/cardinality-runtime.md for allowlist runtime and lint enforcement" -p 0 --labels docs,cardinality,instruments --description "Reservation: docs/cardinality-runtime.md. Define public recording API, allowed/prohibited label behavior, unknown metric behavior, log-and-drop policy, and lint/test helper shape before implementation." --acceptance "docs/cardinality-runtime.md specifies wrapper APIs, default log-and-drop behavior, once-per-key versus every-call logging decision, unknown metric handling, and unit/lint test requirements." --silent)

DOC_USAGE=$(bd create "Design docs/usage-boundary.md for normalized usage and Eino boundary" -p 0 --labels docs,usage,migrate-advisor,migrate-symphony --description "Reservation: docs/usage-boundary.md. Define Usage{InputTokens, OutputTokens, Available}, Available=false behavior, and the boundary between core agent-otel and framework/provider-specific usage extraction." --acceptance "docs/usage-boundary.md includes advisor Provider.Advise and symphony Eino graph examples while keeping provider-specific extraction out of core; it decides whether an optional agentotel/eino adapter is justified." --silent)

DOC_REDACTION=$(bd create "Design docs/redaction.md for PromptRedactor and opt-in payload capture" -p 0 --labels docs,privacy,genai-semconv --description "Reservation: docs/redaction.md. Define prompt/completion event capture defaults, PromptRedactor interface, redaction ordering, error handling, and privacy test expectations." --acceptance "docs/redaction.md states payload capture is off by default, redactor runs before span events are attached, unredacted payloads are never logged, and tests cover redaction and disabled capture." --silent)

DOC_ENV=$(bd create "Document docs/env-vars.md with OTel-spec endpoint/header/protocol precedence" -p 0 --labels docs,core --description "Reservation: docs/env-vars.md. Produce operator-facing env-var precedence documentation for traces, metrics, logs, headers, TLS/insecure, protocol, Datadog preset, and dev sink behavior." --acceptance "docs/env-vars.md contains per-signal-over-general examples for OTEL_EXPORTER_OTLP_* variables, gRPC and HTTP examples, Datadog preset examples, and local lotel dev-sink examples." --silent)
bd dep add $DOC_ENV $DOC_BOOTSTRAP

# ========================================
# Phase 2: Test Harnesses
# ========================================

TEST_OTLP_RECEIVER=$(bd create "Build in-process OTLP receiver test harness" -p 0 --labels testing,core,genai-semconv --description "Reservation: internal/otlptest/** or otlptest/**, *_test.go. Implement an in-process OTLP trace/metric/log receiver for deterministic assertions without external processes." --acceptance "go test ./... passes; tests can capture spans, metrics, logs, resource attrs, and raw attribute names for later Datadog/OpenLLMetry/GenAI assertions." --silent)
bd dep add $TEST_OTLP_RECEIVER $SETUP_DEPS
bd dep add $TEST_OTLP_RECEIVER $DOC_GENAI

TEST_LOTEL_HELPER=$(bd create "Lift/adapt lotel integration helper from local-symphony" -p 1 --labels testing,lotel-devsink --description "Reservation: test/integration/lotelhelper/**, integration-tagged tests. Adapt ~/git/local-symphony/test/integration/lotelhelper pattern for this repo with unique service names, ForceFlush before Shutdown, lotel-cli ingest, and DuckDB queries." --acceptance "go test -tags=integration ./test/integration/... can start or connect to lotel, query by service/since, and skip clearly when lotel prerequisites are unavailable." --silent)
bd dep add $TEST_LOTEL_HELPER $SETUP_DEPS

# ========================================
# Phase 3: Core Bootstrap
# ========================================

CORE_OPTIONS=$(bd create "Implement Options, Providers, Init signature, and no-op fallback surface" -p 0 --labels core --description "Reservation: options.go, providers.go, bootstrap.go, cleanup.go, *_test.go for these files. Implement agent_otel.Init(ctx, Options) returning Providers, shutdown, error-compatible behavior and a graceful no-op fallback when exporters cannot be reached." --acceptance "go test ./... passes; public API includes Init(ctx, Options), Providers, ForceFlush, Shutdown-compatible cleanup; tests cover no collector reachable without panics." --silent)
bd dep add $CORE_OPTIONS $SETUP_DEPS
bd dep add $CORE_OPTIONS $DOC_BOOTSTRAP
bd dep add $CORE_OPTIONS $TEST_OTLP_RECEIVER

CORE_ENV=$(bd create "Implement OTel-spec env-var resolution for OTLP traces, metrics, and logs" -p 0 --labels core,testing --description "Reservation: config.go, env.go, config_test.go, env_test.go. Implement per-signal endpoint/header/insecure/protocol precedence over general OTEL_EXPORTER_OTLP_* variables, including scheme-derived TLS and gRPC/HTTP URL normalization." --acceptance "go test ./... passes; table tests cover trace/metric/log per-signal precedence, general fallback, headers, insecure/TLS, protocol selection, and malformed configuration errors." --silent)
bd dep add $CORE_ENV $CORE_OPTIONS
bd dep add $CORE_ENV $DOC_ENV

CORE_EXPORTERS=$(bd create "Wire OTLP gRPC and HTTP exporters for traces, metrics, and logs" -p 0 --labels core,testing --description "Reservation: exporters.go, bootstrap.go, exporters_test.go. Compose trace, metric, and log providers with OTLP gRPC or HTTP exporters selected from options/env; default to async batch where applicable." --acceptance "go test ./... passes; in-process OTLP tests observe trace, metric, and log export over selected protocols; failure paths return actionable errors or documented no-op fallback." --silent)
bd dep add $CORE_EXPORTERS $CORE_ENV
bd dep add $CORE_EXPORTERS $TEST_OTLP_RECEIVER

CORE_SHUTDOWN=$(bd create "Implement ForceFlush and Shutdown ordering across providers and exporters" -p 1 --labels core,testing --description "Reservation: cleanup.go, providers.go, cleanup_test.go. Implement deterministic ForceFlush before Shutdown behavior across trace, metric, log, primary exporter, and optional dev sink providers." --acceptance "go test ./... passes; tests assert flush/shutdown order, timeout handling, idempotency, and behavior when one provider returns an error." --silent)
bd dep add $CORE_SHUTDOWN $CORE_EXPORTERS

CORE_SLOG=$(bd create "Wire otelslog bridge with parameterized slog handler setup" -p 1 --labels core,testing --description "Reservation: slog.go, slog_test.go. Lift the useful otelslog bridge behavior from local-symphony and expose it through Providers/options without hard-coding application-specific logger policy." --acceptance "go test ./... passes; slog records are exported as OTel logs with service resource attrs; tests cover disabled/no-op and enabled bridge modes." --silent)
bd dep add $CORE_SLOG $CORE_EXPORTERS

# ========================================
# Phase 4: GenAI Semconv, Instruments, Usage, Privacy
# ========================================

GENAI_CONSTANTS=$(bd create "Implement local GenAI semantic convention constants and literal-name tests" -p 0 --labels genai-semconv,testing --description "Reservation: genai_attrs.go, genai_attrs_test.go. Add local string constants for required gen_ai.* keys unless current Go semconv exposes exact constants; include literal tests to prevent drift." --acceptance "go test ./... passes; tests assert exact strings for gen_ai.request.model, gen_ai.usage.input_tokens, gen_ai.usage.output_tokens, gen_ai.provider.name, gen_ai.operation.name, and gen_ai.system." --silent)
bd dep add $GENAI_CONSTANTS $DOC_GENAI
bd dep add $GENAI_CONSTANTS $SETUP_DEPS

USAGE_HELPERS=$(bd create "Implement normalized Usage recording helpers" -p 0 --labels usage,instruments,testing --description "Reservation: usage.go, usage_test.go. Implement Usage{InputTokens, OutputTokens, Available} and helper behavior for Available=false without depending on Eino." --acceptance "go test ./... passes; tests cover available usage, unavailable usage, zero tokens, negative rejection or normalization per docs/usage-boundary.md, and no core Eino dependency." --silent)
bd dep add $USAGE_HELPERS $DOC_USAGE
bd dep add $USAGE_HELPERS $GENAI_CONSTANTS

CARDINALITY_CORE=$(bd create "Implement cardinality allowlist and opt-in runtime validation gate" -p 0 --labels cardinality,instruments,testing --description "Reservation: cardinality.go, cardinality_test.go. Lift symphony's documented allowlist into machine-enforced validation with the API and log-and-drop behavior chosen in docs/cardinality-runtime.md." --acceptance "go test ./... passes; tests cover allowed labels, prohibited labels, unknown metrics, log-and-drop behavior, and low-allocation disabled mode." --silent)
bd dep add $CARDINALITY_CORE $DOC_CARDINALITY
bd dep add $CARDINALITY_CORE $GENAI_CONSTANTS

INSTRUMENTS=$(bd create "Implement pre-built model-call metric instruments" -p 0 --labels instruments,genai-semconv,cardinality,usage,testing --description "Reservation: instruments.go, instruments_test.go. Implement ModelLatency, UsageInputTokens, UsageOutputTokens, ErrorsByProvider, and FallbackEngaged recorders using GenAI attrs and cardinality validation." --acceptance "go test ./... passes; OTLP receiver tests assert metric names, units, values, gen_ai.* attrs, cardinality filtering, and Usage{Available:false} behavior." --silent)
bd dep add $INSTRUMENTS $USAGE_HELPERS
bd dep add $INSTRUMENTS $CARDINALITY_CORE
bd dep add $INSTRUMENTS $CORE_EXPORTERS

GENAI_SPANS=$(bd create "Implement GenAI span helpers and model/tool/fallback attribute emission" -p 0 --labels genai-semconv,instruments,testing --description "Reservation: spans.go, spans_test.go. Implement generic span helper APIs for model calls, agent operations, tool calls, fallback, and errors using OTel GenAI naming and attrs." --acceptance "go test ./... passes; OTLP receiver tests assert span names, gen_ai.request.model, gen_ai.provider.name, gen_ai.operation.name, gen_ai.system, token attrs, error attrs, and retained product-specific attrs." --silent)
bd dep add $GENAI_SPANS $GENAI_CONSTANTS
bd dep add $GENAI_SPANS $CORE_EXPORTERS
bd dep add $GENAI_SPANS $DOC_MIGRATION

PRIVACY_REDACTOR=$(bd create "Implement PromptRedactor hook and opt-in GenAI payload events" -p 1 --labels privacy,genai-semconv,testing --description "Reservation: redact.go, events.go, redact_test.go, events_test.go. Add opt-in prompt/completion event capture with PromptRedactor applied before span events are attached." --acceptance "go test ./... passes; tests prove capture is off by default, redaction precedes event attachment, redactor errors follow documented behavior, and unredacted payloads are never logged." --silent)
bd dep add $PRIVACY_REDACTOR $DOC_REDACTION
bd dep add $PRIVACY_REDACTOR $GENAI_SPANS

# ========================================
# Phase 5: Presets and Dev Sink
# ========================================

DATADOG_PRESET=$(bd create "Implement WithDatadogPreset for OTLP/HTTP and Datadog LLM defaults" -p 0 --labels datadog,core,genai-semconv,testing --description "Reservation: presets/datadog.go, presets/datadog_test.go. Implement Datadog preset defaults including HTTP exporter preference, DD-API-KEY header injection from env, recommended sampling/semconv settings, and override behavior." --acceptance "go test ./... passes; tests assert headers, endpoint/protocol defaults, opt-in behavior, override precedence, no vendor lock-in for default Init, and on-wire Datadog-required GenAI attrs." --silent)
bd dep add $DATADOG_PRESET $DOC_DATADOG
bd dep add $DATADOG_PRESET $CORE_EXPORTERS
bd dep add $DATADOG_PRESET $GENAI_SPANS
bd dep add $DATADOG_PRESET $INSTRUMENTS

OPENLLMETRY_COMPAT=$(bd create "Implement WithOpenLLMetryCompat legacy attribute emission" -p 1 --labels openllmetry,genai-semconv,testing --description "Reservation: presets/openllmetry.go, presets/openllmetry_test.go. Add optional legacy llm.* and traceloop.* attributes alongside OTel-native GenAI attributes without changing default behavior." --acceptance "go test ./... passes; tests assert default native-only attrs, compat-mode additional attrs, overlap mapping, and no removal of GenAI attributes." --silent)
bd dep add $OPENLLMETRY_COMPAT $DOC_OPENLLMETRY
bd dep add $OPENLLMETRY_COMPAT $GENAI_SPANS
bd dep add $OPENLLMETRY_COMPAT $INSTRUMENTS

LOTEL_REQUEST_DIR=$(bd create "Verify lotel request directory and document cross-repo ask process" -p 0 --labels lotel-request,lotel-devsink,docs --description "Reservation: docs/lotel-requests.md and read/write check of ~/git/lotel/.agents/requests/. Verify the request directory exists or document creation step; do not change Rust source." --acceptance "docs/lotel-requests.md records the request directory path, expected markdown format, and verification result; no Rust files in ~/git/lotel are modified." --silent)

LOTEL_GENAI_REQUEST=$(bd create "File lotel request for GenAI-aware query/index support if needed" -p 1 --labels lotel-request,lotel-devsink,genai-semconv --description "Reservation: ~/git/lotel/.agents/requests/genai-query-support.md and docs/lotel-requests.md. File a markdown request for any lotel-side GenAI-aware view, stable attribute index, or query affordance needed by integration debugging." --acceptance "~/git/lotel/.agents/requests/genai-query-support.md exists if the gap is confirmed; docs/lotel-requests.md links or copies the request; no lotel Rust source is modified." --silent)
bd dep add $LOTEL_GENAI_REQUEST $LOTEL_REQUEST_DIR
bd dep add $LOTEL_GENAI_REQUEST $DOC_GENAI

LOTEL_DEVSINK=$(bd create "Implement WithDevSink(lotelEndpoint) dual-exporter fail-open convenience" -p 0 --labels lotel-devsink,core,testing --description "Reservation: devsink/lotel.go, devsink/lotel_test.go, bootstrap integration tests. Add dual-exporter composition that sends to operator-configured OTLP backend and local lotel, with dev sink failures failing open." --acceptance "go test ./... passes; tests assert primary exporter remains authoritative, missing lotel does not fail Init, ForceFlush/Shutdown includes dev sink in documented order, and endpoint override works." --silent)
bd dep add $LOTEL_DEVSINK $DOC_BOOTSTRAP
bd dep add $LOTEL_DEVSINK $CORE_SHUTDOWN
bd dep add $LOTEL_DEVSINK $LOTEL_REQUEST_DIR

LOTEL_INTEGRATION=$(bd create "Add integration test against real lotel using DuckDB queries" -p 1 --labels lotel-devsink,testing,genai-semconv --description "Reservation: test/integration/**, devsink integration tests. Use the adapted lotel helper to emit spans/metrics/logs through WithDevSink, ForceFlush, ingest/query with lotel-cli, and verify GenAI attributes." --acceptance "go test -tags=integration ./... passes when lotel prerequisites are installed; test skips clearly otherwise; assertions query by unique service.name and since timestamp." --silent)
bd dep add $LOTEL_INTEGRATION $LOTEL_DEVSINK
bd dep add $LOTEL_INTEGRATION $TEST_LOTEL_HELPER
bd dep add $LOTEL_INTEGRATION $GENAI_SPANS
bd dep add $LOTEL_INTEGRATION $INSTRUMENTS

# ========================================
# Phase 6: Optional Adapter and Docs
# ========================================

EINO_ADAPTER_DECISION=$(bd create "Implement optional agentotel/eino adapter only if usage-boundary design justifies it" -p 2 --labels usage,testing --description "Reservation: optional eino/**, eino tests, go.mod/go.sum only if approved by docs/usage-boundary.md. Add an isolated adapter for Eino usage bridging only if duplication remains after provider extraction." --acceptance "Either no adapter is added and docs/usage-boundary.md explains why, or go test ./... passes with Eino dependency isolated to eino/ and no core package import of github.com/cloudwego/eino." --silent)
bd dep add $EINO_ADAPTER_DECISION $DOC_USAGE
bd dep add $EINO_ADAPTER_DECISION $USAGE_HELPERS

DOC_PUBLIC=$(bd create "Write public documentation for env vars, GenAI attrs, presets, cardinality, redaction, and examples" -p 1 --labels docs,core,genai-semconv,datadog,openllmetry,cardinality,privacy --description "Reservation: README.md, docs/*.md examples. Update operator and developer docs from the finalized implementation, including env precedence, GenAI table, Datadog, OpenLLMetry, cardinality budgets, redactor contract, and basic examples." --acceptance "README and docs are current with implemented APIs; examples compile or are covered by go test examples; docs link to filed lotel requests and migration map." --silent)
bd dep add $DOC_PUBLIC $CORE_SHUTDOWN
bd dep add $DOC_PUBLIC $CORE_SLOG
bd dep add $DOC_PUBLIC $INSTRUMENTS
bd dep add $DOC_PUBLIC $GENAI_SPANS
bd dep add $DOC_PUBLIC $DATADOG_PRESET
bd dep add $DOC_PUBLIC $OPENLLMETRY_COMPAT
bd dep add $DOC_PUBLIC $LOTEL_DEVSINK
bd dep add $DOC_PUBLIC $PRIVACY_REDACTOR

# ========================================
# Phase 7: Consumer Migrations
# ========================================

MIGRATE_ADVISOR_BOOTSTRAP=$(bd create "Advisor migration: replace telemetry bootstrap with agent-otel while preserving CLI/serve behavior" -p 0 --labels migrate-advisor,core --description "Reservation: ~/git/advisor/internal/telemetry/**, ~/git/advisor/internal/cli/**, advisor go.mod/go.sum. Cut over InitServe, InitCLI, InitDisabled, ForceFlush, and Shutdown behavior to agent-otel or intentionally documented replacements." --acceptance "In ~/git/advisor, go test ./... passes; CLI timeout behavior is tested; telemetry bootstrap uses agent-otel; old bootstrap code is removed or left only as compatibility-free shim." --silent)
bd dep add $MIGRATE_ADVISOR_BOOTSTRAP $DOC_MIGRATION
bd dep add $MIGRATE_ADVISOR_BOOTSTRAP $CORE_SHUTDOWN
bd dep add $MIGRATE_ADVISOR_BOOTSTRAP $CORE_SLOG
bd dep add $MIGRATE_ADVISOR_BOOTSTRAP $PRE_ADVISOR_ADR

MIGRATE_ADVISOR_CORE=$(bd create "Advisor migration: replace core advise span and metric emission with GenAI helpers" -p 0 --labels migrate-advisor,genai-semconv,instruments,usage --description "Reservation: ~/git/advisor/internal/advisor/core/**, ~/git/advisor/internal/advisor/provider.go, related tests. Replace advisor.handle and advisor.call.duration emission with agent-otel span, usage, latency, token, error, and provider/model helpers." --acceptance "In ~/git/advisor, go test ./... passes; tests assert new GenAI attrs and usage behavior; old advisor-specific telemetry names are removed unless retained as product-specific attrs per migration map." --silent)
bd dep add $MIGRATE_ADVISOR_CORE $MIGRATE_ADVISOR_BOOTSTRAP
bd dep add $MIGRATE_ADVISOR_CORE $GENAI_SPANS
bd dep add $MIGRATE_ADVISOR_CORE $INSTRUMENTS
bd dep add $MIGRATE_ADVISOR_CORE $USAGE_HELPERS

MIGRATE_ADVISOR_MCP=$(bd create "Advisor migration: refresh CLI/MCP integration tests and one-time replay corpus expectations" -p 1 --labels migrate-advisor,testing --description "Reservation: ~/git/advisor/internal/mcp/**, ~/git/advisor/integration/**, replay fixtures, docs/adr references. Regenerate or update MCP/CLI telemetry-sensitive tests for the new semantics without preserving byte-equal old output." --acceptance "In ~/git/advisor, relevant integration and MCP tests pass; replay corpus changes are documented as one-time migration cost under the new ADR." --silent)
bd dep add $MIGRATE_ADVISOR_MCP $MIGRATE_ADVISOR_CORE
bd dep add $MIGRATE_ADVISOR_MCP $PRE_ADVISOR_ADR

MIGRATE_SYMPHONY_OBS=$(bd create "Symphony migration: replace internal/obs implementation with agent-otel helpers" -p 0 --labels migrate-symphony,core,genai-semconv,instruments,cardinality --description "Reservation: ~/git/local-symphony/internal/obs/**, go.mod/go.sum, related obs tests. Cut over symphony obs spans, instruments, cardinality, and slog bridge to agent-otel semantics following sibling plan PR5." --acceptance "In ~/git/local-symphony, go test ./... passes; obs package delegates to agent-otel or is removed; GenAI attrs replace old names per migration map." --silent)
bd dep add $MIGRATE_SYMPHONY_OBS $DOC_MIGRATION
bd dep add $MIGRATE_SYMPHONY_OBS $PRE_SYMPHONY_SEMCONV
bd dep add $MIGRATE_SYMPHONY_OBS $PRE_ENV_RECONCILE
bd dep add $MIGRATE_SYMPHONY_OBS $CORE_SLOG
bd dep add $MIGRATE_SYMPHONY_OBS $GENAI_SPANS
bd dep add $MIGRATE_SYMPHONY_OBS $INSTRUMENTS
bd dep add $MIGRATE_SYMPHONY_OBS $CARDINALITY_CORE

MIGRATE_SYMPHONY_WORKERS=$(bd create "Symphony migration: update worker, model, tool, fallback, and Ollama call sites" -p 0 --labels migrate-symphony,usage,genai-semconv --description "Reservation: ~/git/local-symphony/internal/worker/**, model/tool/fallback call sites, related tests. Replace direct obs calls with agent-otel helpers for Eino graph model/tool/collect steps and Ollama request spans." --acceptance "In ~/git/local-symphony, go test ./... passes; tests assert model.name/direction/model.from/model.to/request_kind/error.kind/tool/fallback attrs map to approved GenAI/product-specific attrs." --silent)
bd dep add $MIGRATE_SYMPHONY_WORKERS $MIGRATE_SYMPHONY_OBS
bd dep add $MIGRATE_SYMPHONY_WORKERS $USAGE_HELPERS

MIGRATE_SYMPHONY_INTEGRATION=$(bd create "Symphony migration: update lotel and collector integration tests for agent-otel output" -p 1 --labels migrate-symphony,testing,lotel-devsink --description "Reservation: ~/git/local-symphony/test/integration/**, docker/otel-collector-config.yaml, docker-compose.yml, related docs/dashboard files. Update integration expectations for GenAI attrs and agent-otel bootstrap behavior." --acceptance "In ~/git/local-symphony, integration tests pass under documented prerequisites; lotel and collector pipelines observe new GenAI attrs; dashboards/docs are updated or follow-up gaps are filed." --silent)
bd dep add $MIGRATE_SYMPHONY_INTEGRATION $MIGRATE_SYMPHONY_WORKERS
bd dep add $MIGRATE_SYMPHONY_INTEGRATION $LOTEL_INTEGRATION

# ========================================
# Phase 8: Final Verification
# ========================================

VERIFY_LIBRARY=$(bd create "Run full agent-otel verification gate" -p 0 --labels testing,setup --description "Reservation: no source files unless fixes are required. Run the complete local verification suite after implementation: unit tests, race tests, lint, and integration tests where prerequisites exist." --acceptance "go test ./... passes; go test -race ./... passes; golangci-lint run ./... passes; go test -tags=integration ./... passes or skips only for documented missing external prerequisites." --silent)
bd dep add $VERIFY_LIBRARY $SETUP_CI
bd dep add $VERIFY_LIBRARY $DOC_PUBLIC
bd dep add $VERIFY_LIBRARY $LOTEL_INTEGRATION
bd dep add $VERIFY_LIBRARY $EINO_ADAPTER_DECISION

VERIFY_CONSUMERS=$(bd create "Run advisor and local-symphony consumer verification after lockstep migration" -p 0 --labels testing,migrate-advisor,migrate-symphony --description "Reservation: no source files unless fixes are required. Run consumer test suites and collect review artifacts showing advisor and local-symphony both compile and emit the new telemetry semantics." --acceptance "In ~/git/advisor and ~/git/local-symphony, go test ./... passes; documented integration suites pass or skip with clear prerequisites; migration PR notes link to docs/telemetry-migration-map.md." --silent)
bd dep add $VERIFY_CONSUMERS $MIGRATE_ADVISOR_MCP
bd dep add $VERIFY_CONSUMERS $MIGRATE_SYMPHONY_INTEGRATION
bd dep add $VERIFY_CONSUMERS $VERIFY_LIBRARY

echo ""
echo "Bead graph created."
echo ""
echo "Operator instructions:"
echo "1. Run it: chmod +x setup-beads.sh && ./setup-beads.sh"
echo "2. Check ready work: bd ready should show initial setup tasks plus the three pre-extraction items from sibling plan §3 (symphony semconv bump, advisor ADR-0009, env-var precedence reconciliation) and the blocker design-doc beads. Those are independent and can run in parallel with agent-otel skeleton work where dependencies allow."
echo ""
echo "Useful commands:"
echo "  bd ready"
echo "  bd list --label core"
echo "  bd list --label migrate-advisor"
echo "  bd list --label migrate-symphony"
