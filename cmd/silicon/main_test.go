package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestEndToEnd_EmitsTaskSpan(t *testing.T) {
	// override dotenvLoad to no-op
	oldDot := dotenvLoad
	dotenvLoad = func(...string) error { return nil }
	defer func() { dotenvLoad = oldDot }()

	// install in-memory exporter via telemetryInit override
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
	oldInit := telemetryInit
	telemetryInit = func(ctx context.Context, cfg telemetry.Config) (func(context.Context) error, error) {
		otel.SetTracerProvider(tp)
		return tp.Shutdown, nil
	}
	defer func() {
		telemetryInit = oldInit
		otel.SetTracerProvider(prev)
	}()

	handler, shutdown, err := setup(context.Background())
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer shutdown(context.Background())

	srv := httptest.NewServer(handler)
	defer srv.Close()

	// create task
	body := api.CreateTaskRequest{TaskID: "task-1", Prompt: "hello"}
	b, _ := json.Marshal(body)
	resp, err := http.Post(srv.URL+"/v1/tasks", "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("post create: %v", err)
	}
	resp.Body.Close()

	// poll for completion
	deadline := time.Now().Add(time.Second)
	for {
		if time.Now().After(deadline) {
			t.Fatalf("task did not reach terminal state in time")
		}
		resp, err := http.Get(srv.URL + "/v1/tasks/task-1")
		if err != nil {
			t.Fatalf("get task: %v", err)
		}
		var got api.Task
		_ = json.NewDecoder(resp.Body).Decode(&got)
		resp.Body.Close()
		if got.Status == "completed" || got.Status == "cancelled" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// ensure spans flushed
	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}

	spans := exp.GetSpans()
	found := false
	for _, s := range spans {
		if s.Name == "silicon.task" {
			for _, a := range s.Attributes {
				if a.Key == attribute.Key("task.id") && a.Value.AsString() == "task-1" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("did not find silicon.task span with task.id")
	}
}
