package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestInit_RequiresServiceName(t *testing.T) {
	_, err := Init(context.Background(), Config{})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNewTracerProviderWithExporter_EmitsSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()

	tp, shutdown, err := newTracerProviderWithExporter(exp, Config{ServiceName: "testsvc", ServiceVersion: "v0"})
	if err != nil {
		t.Fatalf("new tracer provider: %v", err)
	}

	tr := tp.Tracer("test")
	_, sp := tr.Start(context.Background(), "root.span")
	sp.End()

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if spans[0].Name != "root.span" {
		t.Fatalf("unexpected span name: %q", spans[0].Name)
	}

	if spans[0].Resource == nil {
		t.Fatalf("expected span resource")
	}
	attrs := spans[0].Resource.Attributes()
	foundName := false
	for _, kv := range attrs {
		if kv.Key == attribute.Key("service.name") {
			foundName = kv.Value.AsString() == "testsvc"
		}
	}
	if !foundName {
		t.Fatalf("expected resource to include service.name=testsvc")
	}
}
