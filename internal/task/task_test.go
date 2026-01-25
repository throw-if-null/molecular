package task

import (
	"context"
	"testing"

	"github.com/throw-if-null/molecular/internal/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestExecute_EmitsSpans(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	res, err := sdkresource.New(context.Background(), sdkresource.WithAttributes(
		attribute.String("service.name", "testsvc"),
		attribute.String("service.version", "v0"),
	))
	if err != nil {
		t.Fatalf("resource: %v", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exp)),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)
	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	// create a test task that succeeds
	task := api.Task{TaskID: "task-1", Prompt: "do something"}
	ctx := context.Background()

	if err := Execute(ctx, task); err != nil {
		t.Fatalf("execute: %v", err)
	}

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
}

func TestExecute_Cancelled_EmitsCancelledEvent(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exp)),
	)
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)
	defer func() {
		_ = tp.Shutdown(context.Background())
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Execute(ctx, api.Task{TaskID: "task-1", Prompt: "ignored"})
	if err == nil {
		t.Fatalf("expected error")
	}

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}

	spans := exp.GetSpans()
	if len(spans) == 0 {
		t.Fatalf("expected spans, got none")
	}

	foundCancelled := false
	for _, s := range spans {
		if s.Name != "silicon.task" {
			continue
		}
		for _, ev := range s.Events {
			if ev.Name == "task.cancelled" {
				foundCancelled = true
			}
		}
	}
	if !foundCancelled {
		t.Fatalf("expected task.cancelled event")
	}
}
