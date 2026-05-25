package agentotel

import (
	"log/slog"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/attribute"
)

type SlogBridgeOptions struct {
	Version    string
	SchemaURL  string
	Source     bool
	Attributes []attribute.KeyValue
}

type SlogOption func(*SlogBridgeOptions)

func WithSlogVersion(version string) SlogOption {
	return func(opts *SlogBridgeOptions) {
		opts.Version = version
	}
}

func WithSlogSchemaURL(schemaURL string) SlogOption {
	return func(opts *SlogBridgeOptions) {
		opts.SchemaURL = schemaURL
	}
}

func WithSlogSource(source bool) SlogOption {
	return func(opts *SlogBridgeOptions) {
		opts.Source = source
	}
}

func WithSlogAttributes(attrs ...attribute.KeyValue) SlogOption {
	return func(opts *SlogBridgeOptions) {
		opts.Attributes = append(opts.Attributes, attrs...)
	}
}

func (p *Providers) SlogHandler(name string, options ...SlogOption) slog.Handler {
	cfg := SlogBridgeOptions{}
	for _, opt := range options {
		if opt != nil {
			opt(&cfg)
		}
	}

	otelOptions := []otelslog.Option{}
	if p != nil && p.LoggerProvider != nil {
		otelOptions = append(otelOptions, otelslog.WithLoggerProvider(p.LoggerProvider))
	}
	if cfg.Version != "" {
		otelOptions = append(otelOptions, otelslog.WithVersion(cfg.Version))
	}
	if cfg.SchemaURL != "" {
		otelOptions = append(otelOptions, otelslog.WithSchemaURL(cfg.SchemaURL))
	}
	otelOptions = append(otelOptions, otelslog.WithSource(cfg.Source))
	if len(cfg.Attributes) > 0 {
		otelOptions = append(otelOptions, otelslog.WithAttributes(cfg.Attributes...))
	}
	return otelslog.NewHandler(name, otelOptions...)
}

func (p *Providers) SlogLogger(name string, options ...SlogOption) *slog.Logger {
	return slog.New(p.SlogHandler(name, options...))
}
