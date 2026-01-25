package task

import (
	"context"

	"github.com/throw-if-null/molecular/internal/api"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Execute runs a single task lifecycle, emitting tracing spans and events.
// This is intentionally small: it creates a root span for the task and
// emits events for key state transitions so tests and tracing backends can
// observe task progress.
func Execute(ctx context.Context, t api.Task) error {
	tr := otel.Tracer("silicon")
	ctx, span := tr.Start(ctx, "silicon.task", trace.WithAttributes(attribute.String("task.id", t.TaskID)))
	defer span.End()

	// task created
	span.AddEvent("task.created")

	// task started
	span.AddEvent("task.started")

	// simulate a child operation (attempt) so we can exercise context propagation
	_, child := tr.Start(ctx, "silicon.attempt", trace.WithAttributes(attribute.String("task.id", t.TaskID)))
	child.AddEvent("attempt.started")

	// For this minimal implementation, treat a prompt that contains the
	// substring "fail" as an injected error to exercise error recording.
	if t.Prompt != "" && containsFail(t.Prompt) {
		err := &taskError{msg: "simulated failure"}
		child.RecordError(err)
		child.SetStatus(codes.Error, err.Error())
		child.AddEvent("attempt.failed")
		child.End()

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("task.failed")
		return err
	}

	child.AddEvent("attempt.completed")
	child.End()

	// task completed
	span.AddEvent("task.completed")
	span.SetStatus(codes.Ok, "")
	return nil
}

// small helper to avoid importing strings in hot path; trivial and test-friendly
func containsFail(s string) bool {
	// simple case-insensitive check for "fail"
	for i := 0; i+4 <= len(s); i++ {
		sub := s[i : i+4]
		if sub == "fail" || sub == "FAIL" || sub == "Fail" {
			return true
		}
	}
	return false
}

type taskError struct{ msg string }

func (t *taskError) Error() string { return t.msg }
