package server

import (
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
	"github.com/lunarforge/workflow/flowwatch/internal/adapter"
)

func workflowDefToProto(def *adapter.WorkflowDef) *flowwatchv1.WorkflowDefinition {
	pb := &flowwatchv1.WorkflowDefinition{
		Id:          def.ID,
		Name:        def.Name,
		Engine:      def.Engine,
		Version:     def.Version,
		SubsystemId: def.SubsystemID,
		Metadata:    def.Metadata,
	}
	if !def.CreatedAt.IsZero() {
		pb.CreatedAt = timestamppb.New(def.CreatedAt)
	}
	return pb
}

func graphDataToProto(g *adapter.GraphData) *flowwatchv1.Graph {
	var nodes []*flowwatchv1.GraphNode
	for _, n := range g.Nodes {
		nodes = append(nodes, &flowwatchv1.GraphNode{
			Id:       n.ID,
			Label:    n.Label,
			Type:     graphNodeType(n.Type),
			Metadata: n.Metadata,
		})
	}

	var edges []*flowwatchv1.GraphEdge
	for _, e := range g.Edges {
		edges = append(edges, &flowwatchv1.GraphEdge{
			SourceId: e.SourceID,
			TargetId: e.TargetID,
			Label:    e.Label,
		})
	}

	return &flowwatchv1.Graph{
		WorkflowId: g.WorkflowID,
		Nodes:      nodes,
		Edges:      edges,
	}
}

func graphNodeType(t string) flowwatchv1.GraphNodeType {
	switch t {
	case "step":
		return flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_STEP
	case "condition":
		return flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_CONDITION
	case "fork":
		return flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_FORK
	case "join":
		return flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_JOIN
	case "subworkflow":
		return flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_SUBWORKFLOW
	default:
		return flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_STEP
	}
}

func capabilitiesToProto(caps *adapter.EngineCapabilities) *flowwatchv1.Capabilities {
	return &flowwatchv1.Capabilities{
		RetryRun:      caps.RetryRun,
		RetryFromStep: caps.RetryFromStep,
		CancelRun:     caps.CancelRun,
		PauseRun:      caps.PauseRun,
		ResumeRun:     caps.ResumeRun,
		SkipStep:      caps.SkipStep,
		Streaming:     caps.Streaming,
		Analytics:     caps.Analytics,
	}
}

func subsystemSummaryToProto(s *adapter.SubsystemSummary) *flowwatchv1.SubsystemSummary {
	return &flowwatchv1.SubsystemSummary{
		Subsystem: &flowwatchv1.Subsystem{
			Id:          s.Subsystem.ID,
			Name:        s.Subsystem.Name,
			Description: s.Subsystem.Description,
			Metadata:    s.Subsystem.Metadata,
		},
		WorkflowCount:    int32(s.WorkflowCount),
		ActiveRuns:       int32(s.ActiveRuns),
		FailedLastHour:   int32(s.FailedLastHour),
		ErrorRateLastHour: s.ErrorRateLastHr,
	}
}

func runDataToProto(r *adapter.RunData) *flowwatchv1.Run {
	pb := &flowwatchv1.Run{
		Id:          r.ID,
		WorkflowId:  r.WorkflowID,
		ForeignId:   r.ForeignID,
		Status:      runStatusToProto(r.Status),
		CurrentStep: r.CurrentStep,
		SubsystemId: r.SubsystemID,
		Trace: &flowwatchv1.TraceContext{
			TraceId:      r.Trace.TraceID,
			SpanId:       r.Trace.SpanID,
			ParentSpanId: r.Trace.ParentSpanID,
			TraceSource:  r.Trace.TraceSource,
		},
		StartedAt: timestamppb.New(r.StartedAt),
		Duration:  durationpb.New(r.Duration),
		Metadata:  r.Metadata,
	}

	if r.FinishedAt != nil {
		pb.FinishedAt = timestamppb.New(*r.FinishedAt)
	}

	if r.Error != nil {
		pb.Error = errorDataToProto(r.Error)
	}

	return pb
}

func runStatusToProto(status string) flowwatchv1.RunStatus {
	switch status {
	case "pending":
		return flowwatchv1.RunStatus_RUN_STATUS_PENDING
	case "running":
		return flowwatchv1.RunStatus_RUN_STATUS_RUNNING
	case "succeeded":
		return flowwatchv1.RunStatus_RUN_STATUS_SUCCEEDED
	case "failed":
		return flowwatchv1.RunStatus_RUN_STATUS_FAILED
	case "canceled":
		return flowwatchv1.RunStatus_RUN_STATUS_CANCELED
	case "paused":
		return flowwatchv1.RunStatus_RUN_STATUS_PAUSED
	default:
		return flowwatchv1.RunStatus_RUN_STATUS_UNSPECIFIED
	}
}

func errorDataToProto(e *adapter.ErrorData) *flowwatchv1.ErrorDetail {
	if e == nil {
		return nil
	}
	return &flowwatchv1.ErrorDetail{
		Message:    e.Message,
		Code:       e.Code,
		StackTrace: e.StackTrace,
	}
}

func stepDataToProto(s *adapter.StepData) *flowwatchv1.StepExecution {
	pb := &flowwatchv1.StepExecution{
		Id:        s.ID,
		RunId:     s.RunID,
		StepName:  s.StepName,
		Status:    stepStatusToProto(s.Status),
		Attempt:   int32(s.Attempt),
		StartedAt: timestamppb.New(s.StartedAt),
		Duration:  durationpb.New(s.Duration),
		QueueWait: durationpb.New(s.QueueWait),
	}

	if s.FinishedAt != nil {
		pb.FinishedAt = timestamppb.New(*s.FinishedAt)
	}

	if s.Error != nil {
		pb.Error = errorDataToProto(s.Error)
	}

	return pb
}

func stepStatusToProto(status string) flowwatchv1.StepStatus {
	switch status {
	case "pending":
		return flowwatchv1.StepStatus_STEP_STATUS_PENDING
	case "running":
		return flowwatchv1.StepStatus_STEP_STATUS_RUNNING
	case "succeeded":
		return flowwatchv1.StepStatus_STEP_STATUS_SUCCEEDED
	case "failed":
		return flowwatchv1.StepStatus_STEP_STATUS_FAILED
	case "skipped":
		return flowwatchv1.StepStatus_STEP_STATUS_SKIPPED
	case "canceled":
		return flowwatchv1.StepStatus_STEP_STATUS_CANCELED
	default:
		return flowwatchv1.StepStatus_STEP_STATUS_UNSPECIFIED
	}
}

func timelineEntryToProto(e *adapter.TimelineEntry) *flowwatchv1.TimelineEntry {
	return &flowwatchv1.TimelineEntry{
		StepName:  e.StepName,
		Status:    stepStatusToProto(e.Status),
		Attempt:   int32(e.Attempt),
		Offset:    durationpb.New(e.Offset),
		Duration:  durationpb.New(e.Duration),
		QueueWait: durationpb.New(e.QueueWait),
		Error:     errorDataToProto(e.Error),
	}
}

func runFilterFromProto(f *flowwatchv1.RunFilter) adapter.RunFilter {
	if f == nil {
		return adapter.RunFilter{}
	}

	af := adapter.RunFilter{
		WorkflowIDs:  f.WorkflowIds,
		ForeignID:    f.ForeignId,
		SubsystemIDs: f.SubsystemIds,
		TraceID:      f.TraceId,
	}

	for _, s := range f.Statuses {
		af.Statuses = append(af.Statuses, runStatusFromProto(s))
	}

	if f.TimeRange != nil {
		if f.TimeRange.From != nil {
			t := f.TimeRange.From.AsTime()
			af.From = &t
		}
		if f.TimeRange.To != nil {
			t := f.TimeRange.To.AsTime()
			af.To = &t
		}
	}

	af.MetadataMatch = f.MetadataMatch
	return af
}

func runStatusFromProto(s flowwatchv1.RunStatus) string {
	switch s {
	case flowwatchv1.RunStatus_RUN_STATUS_PENDING:
		return "pending"
	case flowwatchv1.RunStatus_RUN_STATUS_RUNNING:
		return "running"
	case flowwatchv1.RunStatus_RUN_STATUS_SUCCEEDED:
		return "succeeded"
	case flowwatchv1.RunStatus_RUN_STATUS_FAILED:
		return "failed"
	case flowwatchv1.RunStatus_RUN_STATUS_CANCELED:
		return "canceled"
	case flowwatchv1.RunStatus_RUN_STATUS_PAUSED:
		return "paused"
	default:
		return ""
	}
}
