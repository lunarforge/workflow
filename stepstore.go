package workflow

import "context"

// StepStore persists step execution records for observability and monitoring. Implementations
// should be tested with adaptertest.TestStepStore.
//
// StepStore is optional — workflows function without it. When provided via WithStepStore,
// the workflow engine automatically records step execution data (start, end, duration, errors)
// for each step consumer invocation.
type StepStore interface {
	// Create persists a new step execution record and returns the assigned ID.
	Create(ctx context.Context, record *StepRecord) error

	// Update modifies an existing step record, typically to mark it as completed or failed.
	Update(ctx context.Context, record *StepRecord) error

	// List returns all step records for a given run, ordered by StartedAt ascending.
	List(ctx context.Context, workflowName, runID string) ([]StepRecord, error)

	// ListByStatus returns all step records for a given run that match the specified status value.
	ListByStatus(ctx context.Context, workflowName, runID string, status int) ([]StepRecord, error)
}
