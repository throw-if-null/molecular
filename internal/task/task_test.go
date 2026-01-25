package task

import (
	"context"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestExecute_EmitsSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp, shutdown, err := telemetry.NewTracerProviderWithExporter(exp, telemetry.Config{ServiceName: "testsvc", ServiceVersion: "v0"})
	if err != nil {
		t.Fatalf("new tracer provider: %v", err)
	}
	// install tracer provider for global otel
	// reuse telemetry package helpers
	// set as global tracer provider so otel.Tracer picks it up
	// instead of calling otel.SetTracerProvider we reuse the provider directly

	tr := tp.Tracer("silicon")
	// install provider as global so Execute (which uses otel.Tracer) picks it up
	otel.SetTracerProvider(tp)

	// create a test task that succeeds
	task := api.Task{TaskID: "task-1", Prompt: "do something"}
	ctx := context.Background()

	// execute with tracer provider in place by using context
	// start a root using tracer to ensure parent-based sampling works
	_, root := tr.Start(ctx, "test.root")
	if err := Execute(ctx, task); err != nil {
		t.Fatalf("execute: %v", err)
	}
	root.End()

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatalf("expected spans, got none")
	}

	// find a span named silicon.task
	found := false
	for _, s := range spans {
		if s.Name == "silicon.task" {
			found = true
			// expect resource contains service.name
			attrs := s.Resource.Attributes()
			gotSvc := false
			for _, kv := range attrs {
				if kv.Key == attribute.Key("service.name") && kv.Value.AsString() == "testsvc" {
					gotSvc = true
				}
			}
			if !gotSvc {
				t.Fatalf("expected span resource to contain service.name=testsvc")
			}
			// check attribute task.id
			hasTaskID := false
			for _, kv := range s.Attributes {
				if kv.Key == attribute.Key("task.id") && kv.Value.AsString() == "task-1" {
					hasTaskID = true
				}
			}
			if !hasTaskID {
				t.Fatalf("expected span to include task.id attribute")
			}
		}
	}
	if !found {
		t.Fatalf("did not find silicon.task span")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
}
