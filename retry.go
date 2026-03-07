package workflow

import (
	"context"
	"fmt"
)

// Retry creates a new workflow run for the same foreignID as the specified run, re-using its Object
// and Metadata. The previous run must be in a finished state. Additional TriggerOptions can override
// the starting point, initial value, or metadata for the retry.
func (w *Workflow[Type, Status]) Retry(
	ctx context.Context,
	runID string,
	opts ...TriggerOption[Type, Status],
) (newRunID string, err error) {
	record, err := w.recordStore.Lookup(ctx, runID)
	if err != nil {
		return "", fmt.Errorf("retry failed: %w", err)
	}

	if !record.RunState.Finished() {
		return "", fmt.Errorf("retry failed: run %s is not in a finished state (current: %s)", runID, record.RunState)
	}

	var t Type
	err = Unmarshal(record.Object, &t)
	if err != nil {
		return "", fmt.Errorf("retry failed: unmarshal object: %w", err)
	}

	// Build options: carry forward the previous run's object and metadata by default,
	// but allow the caller to override via opts.
	defaultOpts := []TriggerOption[Type, Status]{
		WithInitialValue[Type, Status](&t),
		WithMetadata[Type, Status](record.Metadata),
	}

	// If the original run had trace context, carry it forward as parent span.
	if record.Meta.TraceID != "" {
		defaultOpts = append(defaultOpts, WithTraceContext[Type, Status](
			record.Meta.TraceID,
			"",
			record.Meta.SpanID,
		))
	}

	// User-provided opts come last and override defaults.
	allOpts := append(defaultOpts, opts...)

	return w.Trigger(ctx, record.ForeignID, allOpts...)
}
