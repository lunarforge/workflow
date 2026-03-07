package workflow

import "time"

// StepStatus represents the execution state of a step.
type StepStatus int

const (
	StepStatusUnknown   StepStatus = 0
	StepStatusStarted   StepStatus = 1
	StepStatusSucceeded StepStatus = 2
	StepStatusFailed    StepStatus = 3
	StepStatusSkipped   StepStatus = 4
)

func (s StepStatus) String() string {
	switch s {
	case StepStatusStarted:
		return "Started"
	case StepStatusSucceeded:
		return "Succeeded"
	case StepStatusFailed:
		return "Failed"
	case StepStatusSkipped:
		return "Skipped"
	default:
		return "Unknown"
	}
}

// StepRecord captures a single execution of a workflow step. Each retry attempt creates a new
// StepRecord with an incremented Attempt number.
type StepRecord struct {
	// ID uniquely identifies this step execution record.
	ID int64

	// WorkflowName is the name of the workflow this step belongs to.
	WorkflowName string

	// ForeignID is the external identifier linking to the workflow run's foreign entity.
	ForeignID string

	// RunID is the UUID of the workflow run this step belongs to.
	RunID string

	// Status is the integer status value being consumed (the "from" status of the step).
	Status int

	// StatusDescription is the human-readable name of the status being consumed.
	StatusDescription string

	// StepStatus tracks the execution state of this step (started, succeeded, failed, skipped).
	StepStatus StepStatus

	// Attempt is the retry attempt number (1-based). Each retry creates a new StepRecord.
	Attempt int

	// StartedAt is when this step execution began.
	StartedAt time.Time

	// FinishedAt is when this step execution completed (nil if still running).
	FinishedAt *time.Time

	// Duration is the wall-clock time the step took. Set when the step completes.
	Duration time.Duration

	// ToStatus is the status the step transitioned to on success. Zero if the step failed or is still running.
	ToStatus int

	// ErrorMessage captures the error message if the step failed.
	ErrorMessage string
}
