package server

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
)

func (s *workflowServer) ListWorkflows(ctx context.Context, req *connect.Request[flowwatchv1.ListWorkflowsRequest]) (*connect.Response[flowwatchv1.ListWorkflowsResponse], error) {
	workflows, err := s.adapter.ListWorkflows(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var pbs []*flowwatchv1.WorkflowDefinition
	for i := range workflows {
		pbs = append(pbs, workflowDefToProto(&workflows[i]))
	}

	return connect.NewResponse(&flowwatchv1.ListWorkflowsResponse{
		Workflows: pbs,
	}), nil
}

func (s *workflowServer) GetWorkflow(ctx context.Context, req *connect.Request[flowwatchv1.GetWorkflowRequest]) (*connect.Response[flowwatchv1.GetWorkflowResponse], error) {
	wf, err := s.adapter.GetWorkflow(ctx, req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&flowwatchv1.GetWorkflowResponse{
		Workflow: workflowDefToProto(wf),
	}), nil
}

func (s *workflowServer) GetGraph(ctx context.Context, req *connect.Request[flowwatchv1.GetGraphRequest]) (*connect.Response[flowwatchv1.GetGraphResponse], error) {
	graph, err := s.adapter.GetGraph(ctx, req.Msg.WorkflowId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&flowwatchv1.GetGraphResponse{
		Graph: graphDataToProto(graph),
	}), nil
}

func (s *workflowServer) GetCapabilities(_ context.Context, _ *connect.Request[flowwatchv1.GetCapabilitiesRequest]) (*connect.Response[flowwatchv1.GetCapabilitiesResponse], error) {
	caps := s.adapter.Capabilities()
	return connect.NewResponse(&flowwatchv1.GetCapabilitiesResponse{
		Capabilities: capabilitiesToProto(caps),
	}), nil
}

func (s *workflowServer) ListSubsystems(ctx context.Context, _ *connect.Request[flowwatchv1.ListSubsystemsRequest]) (*connect.Response[flowwatchv1.ListSubsystemsResponse], error) {
	subs, err := s.adapter.ListSubsystems(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var pbs []*flowwatchv1.SubsystemSummary
	for i := range subs {
		pbs = append(pbs, subsystemSummaryToProto(&subs[i]))
	}

	return connect.NewResponse(&flowwatchv1.ListSubsystemsResponse{
		Subsystems: pbs,
	}), nil
}

func (s *workflowServer) GetSubsystem(ctx context.Context, req *connect.Request[flowwatchv1.GetSubsystemRequest]) (*connect.Response[flowwatchv1.GetSubsystemResponse], error) {
	detail, err := s.adapter.GetSubsystem(ctx, req.Msg.SubsystemId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	var wfPbs []*flowwatchv1.WorkflowDefinition
	for i := range detail.Workflows {
		wfPbs = append(wfPbs, workflowDefToProto(&detail.Workflows[i]))
	}

	return connect.NewResponse(&flowwatchv1.GetSubsystemResponse{
		Subsystem: subsystemSummaryToProto(&detail.Summary),
		Workflows: wfPbs,
	}), nil
}

func (s *workflowServer) GetTrace(ctx context.Context, req *connect.Request[flowwatchv1.GetTraceRequest]) (*connect.Response[flowwatchv1.GetTraceResponse], error) {
	trace, err := s.adapter.GetTrace(ctx, req.Msg.TraceId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	var spans []*flowwatchv1.TraceSpan
	for _, span := range trace.Spans {
		var stepPbs []*flowwatchv1.StepExecution
		for i := range span.Steps {
			stepPbs = append(stepPbs, stepDataToProto(&span.Steps[i]))
		}

		spans = append(spans, &flowwatchv1.TraceSpan{
			SpanId:       span.SpanID,
			ParentSpanId: span.ParentSpanID,
			Run:          runDataToProto(&span.Run),
			SubsystemId:  span.SubsystemID,
			Offset:       durationpb.New(span.Offset),
			Duration:     durationpb.New(span.Duration),
			Steps:        stepPbs,
		})
	}

	return connect.NewResponse(&flowwatchv1.GetTraceResponse{
		TraceId:        trace.TraceID,
		Spans:          spans,
		TotalDuration:  durationpb.New(trace.TotalDuration),
		SubsystemCount: int32(trace.SubsystemCount),
	}), nil
}
