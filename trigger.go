package workflow

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/lunarforge/workflow/internal/stack"
	"github.com/lunarforge/workflow/internal/util"
)

func (w *Workflow[Type, Status]) Trigger(
	ctx context.Context,
	foreignID string,
	opts ...TriggerOption[Type, Status],
) (runID string, err error) {
	return trigger(ctx, w, w.recordStore.Latest, w.recordStore.Store, foreignID, opts...)
}

func trigger[Type any, Status StatusType](
	ctx context.Context,
	w *Workflow[Type, Status],
	lookup latestLookup,
	store storeFunc,
	foreignID string,
	opts ...TriggerOption[Type, Status],
) (runID string, err error) {
	if !w.calledRun {
		return "", fmt.Errorf("trigger failed: workflow is not running")
	}

	var o triggerOpts[Type, Status]
	for _, fn := range opts {
		fn(&o)
	}

	startingStatus := w.defaultStartingPoint
	if o.startingPoint != Status(0) {
		startingStatus = o.startingPoint
	}

	if !w.statusGraph.IsValid(int(startingStatus)) {
		w.logger.Debug(
			w.ctx,
			fmt.Sprintf("ensure %v is configured for workflow: %v", startingStatus, w.Name()),
			map[string]string{},
		)

		return "", fmt.Errorf("trigger failed: status provided is not configured for workflow: %s", startingStatus)
	}

	var t Type
	if o.initialValue != nil {
		t = *o.initialValue
	}

	object, err := Marshal(&t)
	if err != nil {
		return "", err
	}

	lastRecord, err := lookup(ctx, w.Name(), foreignID)
	if errors.Is(err, ErrRecordNotFound) {
		lastRecord = &Record{}
	} else if err != nil {
		return "", err
	}

	// Check that the last run has completed before triggering a new run.
	if lastRecord.RunState.Valid() && !lastRecord.RunState.Finished() {
		// Cannot trigger a new run for this foreignID if there is a workflow in progress.
		return "", ErrWorkflowInProgress
	}

	meta := Meta{
		StatusDescription: util.CamelCaseToSpacing(startingStatus.String()),
		TraceOrigin:       stack.Trace(),
		TraceID:           o.traceID,
		SpanID:            o.spanID,
		ParentSpanID:      o.parentSpanID,
	}

	uid, err := uuid.NewUUID()
	if err != nil {
		return "", err
	}

	runID = uid.String()
	wr := &Record{
		WorkflowName: w.Name(),
		ForeignID:    foreignID,
		RunID:        runID,
		RunState:     RunStateInitiated,
		Status:       int(startingStatus),
		Object:       object,
		CreatedAt:    w.clock.Now(),
		UpdatedAt:    w.clock.Now(),
		Meta:         meta,
		Metadata:     mergeMetadata(w.defaultMetadata, o.metadata),
	}

	err = updateRecord(ctx, store, wr, RunStateUnknown, startingStatus.String())
	if err != nil {
		return "", err
	}

	return runID, nil
}

type triggerOpts[Type any, Status StatusType] struct {
	startingPoint Status
	initialValue  *Type
	metadata      map[string]string
	traceID       string
	spanID        string
	parentSpanID  string
}

type TriggerOption[Type any, Status StatusType] func(o *triggerOpts[Type, Status])

func WithStartingPoint[Type any, Status StatusType](startingStatus Status) TriggerOption[Type, Status] {
	return func(o *triggerOpts[Type, Status]) {
		o.startingPoint = startingStatus
	}
}

func WithInitialValue[Type any, Status StatusType](t *Type) TriggerOption[Type, Status] {
	return func(o *triggerOpts[Type, Status]) {
		o.initialValue = t
	}
}

// WithMetadata attaches arbitrary key-value pairs to the workflow record. Metadata is persisted
// alongside the record and can be used for tagging, grouping, filtering, or attaching domain-specific
// context such as subsystem identifiers or trace IDs.
func WithMetadata[Type any, Status StatusType](metadata map[string]string) TriggerOption[Type, Status] {
	return func(o *triggerOpts[Type, Status]) {
		o.metadata = metadata
	}
}

// WithTraceContext attaches distributed tracing identifiers to the workflow run. These are stored
// in the record's Meta and can be used by monitoring tools like FlowWatch to correlate workflow
// runs with external traces across services and subsystems.
func WithTraceContext[Type any, Status StatusType](traceID, spanID, parentSpanID string) TriggerOption[Type, Status] {
	return func(o *triggerOpts[Type, Status]) {
		o.traceID = traceID
		o.spanID = spanID
		o.parentSpanID = parentSpanID
	}
}
