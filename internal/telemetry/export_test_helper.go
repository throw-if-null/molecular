package telemetry

import (
	"context"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// NewTracerProviderWithExporter is an exported wrapper around the internal
// newTracerProviderWithExporter helper so tests in other packages can create
// tracer providers backed by in-memory exporters.
func NewTracerProviderWithExporter(exporter sdktrace.SpanExporter, cfg Config) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	return newTracerProviderWithExporter(exporter, cfg)
}
