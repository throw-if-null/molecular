package telemetry

import (
	"context"
	"errors"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otlptracehttp "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Config controls telemetry initialization behavior.
type Config struct {
	ServiceName    string
	ServiceVersion string
	OTLPEndpoint   string
	Insecure       bool
}

// Init initializes OpenTelemetry tracing using an OTLP/HTTP exporter.
// It sets global propagators and the global TracerProvider. Returns a
// shutdown function that will attempt to flush and stop the provider.
func Init(ctx context.Context, cfg Config) (func(context.Context) error, error) {
	if cfg.ServiceName == "" {
		return nil, errors.New("service name required")
	}

	ep := cfg.OTLPEndpoint
	if ep == "" {
		ep = "http://127.0.0.1:4318"
	}

	u, err := url.Parse(ep)
	if err != nil {
		return nil, err
	}

	endpoint := u.Host
	if endpoint == "" {
		// fallback if user passed host:port without scheme
		endpoint = u.Path
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpoint(endpoint)}
	if cfg.Insecure || u.Scheme == "http" {
		opts = append(opts, otlptracehttp.WithInsecure())
	}

	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, err
	}

	tp, shutdown, err := newTracerProviderWithExporter(exporter, cfg)
	if err != nil {
		// best-effort cleanup of exporter
		_ = exporter.Shutdown(ctx)
		return nil, err
	}

	// set global propagator and tracer provider
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	otel.SetTracerProvider(tp)

	return shutdown, nil
}

// newTracerProviderWithExporter creates a TracerProvider wired to the
// provided SpanExporter. This helper is unexported to allow tests to
// supply in-memory exporters.
func newTracerProviderWithExporter(exporter sdktrace.SpanExporter, cfg Config) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	// resource with basic service attributes
	res, err := sdkresource.New(context.Background(), sdkresource.WithAttributes(
		attribute.String("service.name", cfg.ServiceName),
		attribute.String("service.version", cfg.ServiceVersion),
	))
	if err != nil {
		return nil, nil, err
	}

	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	shutdown := func(ctx context.Context) error {
		return tp.Shutdown(ctx)
	}
	return tp, shutdown, nil
}
