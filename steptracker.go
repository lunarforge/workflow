package workflow

import (
	"context"

	"k8s.io/utils/clock"
)

// stepTracker wraps a StepStore and clock to record step execution data. When the StepStore is nil,
// all methods are no-ops, allowing step tracking to be entirely optional.
type stepTracker struct {
	store StepStore
	clock clock.Clock
}

// begin creates a new StepRecord marked as started and persists it. Returns the record so it can
// be updated when the step completes. Returns nil if tracking is disabled.
func (st *stepTracker) begin(ctx context.Context, workflowName, foreignID, runID string, status int, statusDescription string) *StepRecord {
	if st == nil || st.store == nil {
		return nil
	}

	now := st.clock.Now()
	rec := &StepRecord{
		WorkflowName:      workflowName,
		ForeignID:         foreignID,
		RunID:             runID,
		Status:            status,
		StatusDescription: statusDescription,
		StepStatus:        StepStatusStarted,
		Attempt:           1,
		StartedAt:         now,
	}

	// Best-effort: count previous attempts for this run+status
	existing, err := st.store.ListByStatus(ctx, workflowName, runID, status)
	if err == nil {
		rec.Attempt = len(existing) + 1
	}

	err = st.store.Create(ctx, rec)
	if err != nil {
		// Step tracking is observability — don't fail the step on tracking errors
		return nil
	}

	return rec
}

// succeed marks a step record as succeeded with the given destination status.
func (st *stepTracker) succeed(ctx context.Context, rec *StepRecord, toStatus int) {
	if st == nil || st.store == nil || rec == nil {
		return
	}

	now := st.clock.Now()
	rec.StepStatus = StepStatusSucceeded
	rec.FinishedAt = &now
	rec.Duration = now.Sub(rec.StartedAt)
	rec.ToStatus = toStatus

	// Best-effort update
	_ = st.store.Update(ctx, rec)
}

// fail marks a step record as failed with the given error message.
func (st *stepTracker) fail(ctx context.Context, rec *StepRecord, errMsg string) {
	if st == nil || st.store == nil || rec == nil {
		return
	}

	now := st.clock.Now()
	rec.StepStatus = StepStatusFailed
	rec.FinishedAt = &now
	rec.Duration = now.Sub(rec.StartedAt)
	rec.ErrorMessage = errMsg

	// Best-effort update
	_ = st.store.Update(ctx, rec)
}

// skip marks a step record as skipped.
func (st *stepTracker) skip(ctx context.Context, rec *StepRecord) {
	if st == nil || st.store == nil || rec == nil {
		return
	}

	now := st.clock.Now()
	rec.StepStatus = StepStatusSkipped
	rec.FinishedAt = &now
	rec.Duration = now.Sub(rec.StartedAt)

	// Best-effort update
	_ = st.store.Update(ctx, rec)
}

func newStepTracker(store StepStore, c clock.Clock) *stepTracker {
	if store == nil {
		return nil
	}
	return &stepTracker{store: store, clock: c}
}
