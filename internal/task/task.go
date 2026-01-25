package task

import (
	"context"
	"errors"

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
	ctx, span := tr.Start(
		ctx,
		"silicon.task",
		trace.WithNewRoot(),
		trace.WithAttributes(attribute.String("task.id", t.TaskID)),
	)
	defer span.End()

	// task created
	span.AddEvent("task.created")

	// task started
	span.AddEvent("task.started")

	// child operation (attempt) to exercise context propagation
	_, child := tr.Start(ctx, "silicon.attempt", trace.WithAttributes(attribute.String("task.id", t.TaskID)))
	child.AddEvent("attempt.started")
	select {
	case <-ctx.Done():
		err := ctx.Err()
		if err == nil {
			err = errors.New("context cancelled")
		}
		child.RecordError(err)
		child.SetStatus(codes.Error, err.Error())
		child.AddEvent("attempt.failed")
		child.End()

		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		span.AddEvent("task.cancelled")
		return err
	default:
	}

	child.AddEvent("attempt.completed")
	child.End()

	// task completed
	span.AddEvent("task.completed")
	span.SetStatus(codes.Ok, "")
	return nil
}
