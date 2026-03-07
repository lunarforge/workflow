package lunoworkflow

import (
	"github.com/luno/workflow"

	"github.com/luno/workflow/flowwatch/internal/adapter"
)

func recordToRunData(rec *workflow.Record) adapter.RunData {
	run := adapter.RunData{
		ID:          rec.RunID,
		WorkflowID:  rec.WorkflowName,
		ForeignID:   rec.ForeignID,
		SubsystemID: rec.Subsystem(),
		Status:      runStateToStatus(rec.RunState),
		CurrentStep: rec.Meta.StatusDescription,
		Trace: adapter.TraceContextData{
			TraceID:      rec.Meta.TraceID,
			SpanID:       rec.Meta.SpanID,
			ParentSpanID: rec.Meta.ParentSpanID,
		},
		StartedAt: rec.CreatedAt,
		Duration:  rec.UpdatedAt.Sub(rec.CreatedAt),
		Metadata:  rec.Metadata,
	}

	if rec.RunState.Finished() {
		t := rec.UpdatedAt
		run.FinishedAt = &t
	}

	return run
}

func stepRecordToStepData(s *workflow.StepRecord) adapter.StepData {
	sd := adapter.StepData{
		RunID:    s.RunID,
		StepName: s.StatusDescription,
		Status:   stepStatusToString(s.StepStatus),
		Attempt:  s.Attempt,
		StartedAt: s.StartedAt,
		Duration: s.Duration,
	}

	if s.FinishedAt != nil {
		sd.FinishedAt = s.FinishedAt
	}

	if s.ErrorMessage != "" {
		sd.Error = &adapter.ErrorData{
			Message: s.ErrorMessage,
		}
	}

	return sd
}

func runStateToStatus(rs workflow.RunState) string {
	switch rs {
	case workflow.RunStateInitiated:
		return "pending"
	case workflow.RunStateRunning:
		return "running"
	case workflow.RunStateCompleted:
		return "succeeded"
	case workflow.RunStateCancelled:
		return "canceled"
	case workflow.RunStatePaused:
		return "paused"
	case workflow.RunStateDataDeleted, workflow.RunStateRequestedDataDeleted:
		return "succeeded"
	default:
		return "unknown"
	}
}

func runStatusToRunState(status string) workflow.RunState {
	switch status {
	case "pending":
		return workflow.RunStateInitiated
	case "running":
		return workflow.RunStateRunning
	case "succeeded":
		return workflow.RunStateCompleted
	case "failed":
		return workflow.RunStateCancelled
	case "canceled":
		return workflow.RunStateCancelled
	case "paused":
		return workflow.RunStatePaused
	default:
		return workflow.RunStateUnknown
	}
}

func stepStatusToString(ss workflow.StepStatus) string {
	switch ss {
	case workflow.StepStatusStarted:
		return "running"
	case workflow.StepStatusSucceeded:
		return "succeeded"
	case workflow.StepStatusFailed:
		return "failed"
	case workflow.StepStatusSkipped:
		return "skipped"
	default:
		return "unknown"
	}
}
