package server

import (
	"context"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
)

func (s *runServer) ListRuns(ctx context.Context, req *connect.Request[flowwatchv1.ListRunsRequest]) (*connect.Response[flowwatchv1.ListRunsResponse], error) {
	filter := runFilterFromProto(req.Msg.Filter)

	cursor := ""
	limit := 20
	if req.Msg.Pagination != nil {
		cursor = req.Msg.Pagination.Cursor
		if req.Msg.Pagination.Limit > 0 {
			limit = int(req.Msg.Pagination.Limit)
		}
	}

	runs, nextCursor, err := s.adapter.ListRuns(ctx, filter, cursor, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var pbs []*flowwatchv1.Run
	for i := range runs {
		pbs = append(pbs, runDataToProto(&runs[i]))
	}

	return connect.NewResponse(&flowwatchv1.ListRunsResponse{
		Runs: pbs,
		Pagination: &flowwatchv1.PaginationResponse{
			NextCursor: nextCursor,
			TotalCount: int32(len(pbs)),
		},
	}), nil
}

func (s *runServer) GetRun(ctx context.Context, req *connect.Request[flowwatchv1.GetRunRequest]) (*connect.Response[flowwatchv1.GetRunResponse], error) {
	run, err := s.adapter.GetRun(ctx, req.Msg.RunId)
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, err)
	}

	return connect.NewResponse(&flowwatchv1.GetRunResponse{
		Run: runDataToProto(run),
	}), nil
}

func (s *runServer) GetRunSteps(ctx context.Context, req *connect.Request[flowwatchv1.GetRunStepsRequest]) (*connect.Response[flowwatchv1.GetRunStepsResponse], error) {
	steps, err := s.adapter.GetRunSteps(ctx, req.Msg.RunId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var pbs []*flowwatchv1.StepExecution
	for i := range steps {
		pbs = append(pbs, stepDataToProto(&steps[i]))
	}

	return connect.NewResponse(&flowwatchv1.GetRunStepsResponse{
		Steps: pbs,
	}), nil
}

func (s *runServer) GetRunTimeline(ctx context.Context, req *connect.Request[flowwatchv1.GetRunTimelineRequest]) (*connect.Response[flowwatchv1.GetRunTimelineResponse], error) {
	timeline, err := s.adapter.GetRunTimeline(ctx, req.Msg.RunId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var entries []*flowwatchv1.TimelineEntry
	for i := range timeline.Entries {
		entries = append(entries, timelineEntryToProto(&timeline.Entries[i]))
	}

	return connect.NewResponse(&flowwatchv1.GetRunTimelineResponse{
		RunId:         timeline.RunID,
		TotalDuration: durationpb.New(timeline.TotalDuration),
		Entries:       entries,
	}), nil
}

func (s *runServer) GetRunLogs(ctx context.Context, req *connect.Request[flowwatchv1.GetRunLogsRequest]) (*connect.Response[flowwatchv1.GetRunLogsResponse], error) {
	cursor := ""
	limit := 100
	if req.Msg.Pagination != nil {
		cursor = req.Msg.Pagination.Cursor
		if req.Msg.Pagination.Limit > 0 {
			limit = int(req.Msg.Pagination.Limit)
		}
	}

	_, _, err := s.adapter.GetRunLogs(ctx, req.Msg.RunId, req.Msg.StepName, cursor, limit)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}

	return connect.NewResponse(&flowwatchv1.GetRunLogsResponse{}), nil
}

func (s *runServer) RetryRun(ctx context.Context, req *connect.Request[flowwatchv1.RetryRunRequest]) (*connect.Response[flowwatchv1.RetryRunResponse], error) {
	newRunID, err := s.adapter.RetryRun(ctx, req.Msg.RunId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowwatchv1.RetryRunResponse{
		NewRunId: newRunID,
	}), nil
}

func (s *runServer) RetryFromStep(ctx context.Context, req *connect.Request[flowwatchv1.RetryFromStepRequest]) (*connect.Response[flowwatchv1.RetryFromStepResponse], error) {
	newRunID, err := s.adapter.RetryFromStep(ctx, req.Msg.RunId, req.Msg.StepName)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}

	return connect.NewResponse(&flowwatchv1.RetryFromStepResponse{
		NewRunId: newRunID,
	}), nil
}

func (s *runServer) CancelRun(ctx context.Context, req *connect.Request[flowwatchv1.CancelRunRequest]) (*connect.Response[flowwatchv1.CancelRunResponse], error) {
	err := s.adapter.CancelRun(ctx, req.Msg.RunId, req.Msg.Reason)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowwatchv1.CancelRunResponse{}), nil
}

func (s *runServer) PauseRun(ctx context.Context, req *connect.Request[flowwatchv1.PauseRunRequest]) (*connect.Response[flowwatchv1.PauseRunResponse], error) {
	err := s.adapter.PauseRun(ctx, req.Msg.RunId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowwatchv1.PauseRunResponse{}), nil
}

func (s *runServer) ResumeRun(ctx context.Context, req *connect.Request[flowwatchv1.ResumeRunRequest]) (*connect.Response[flowwatchv1.ResumeRunResponse], error) {
	err := s.adapter.ResumeRun(ctx, req.Msg.RunId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return connect.NewResponse(&flowwatchv1.ResumeRunResponse{}), nil
}

func (s *runServer) SkipStep(ctx context.Context, req *connect.Request[flowwatchv1.SkipStepRequest]) (*connect.Response[flowwatchv1.SkipStepResponse], error) {
	err := s.adapter.SkipStep(ctx, req.Msg.RunId, req.Msg.StepName, req.Msg.Reason)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnimplemented, err)
	}

	return connect.NewResponse(&flowwatchv1.SkipStepResponse{}), nil
}

func (s *runServer) BulkRetry(ctx context.Context, req *connect.Request[flowwatchv1.BulkRetryRequest]) (*connect.Response[flowwatchv1.BulkRetryResponse], error) {
	var results []*flowwatchv1.BulkActionResult
	for _, runID := range req.Msg.RunIds {
		newRunID, err := s.adapter.RetryRun(ctx, runID)
		result := &flowwatchv1.BulkActionResult{
			RunId:    runID,
			Success:  err == nil,
			NewRunId: newRunID,
		}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}

	return connect.NewResponse(&flowwatchv1.BulkRetryResponse{
		Results: results,
	}), nil
}

func (s *runServer) BulkCancel(ctx context.Context, req *connect.Request[flowwatchv1.BulkCancelRequest]) (*connect.Response[flowwatchv1.BulkCancelResponse], error) {
	var results []*flowwatchv1.BulkActionResult
	for _, runID := range req.Msg.RunIds {
		err := s.adapter.CancelRun(ctx, runID, req.Msg.Reason)
		result := &flowwatchv1.BulkActionResult{
			RunId:   runID,
			Success: err == nil,
		}
		if err != nil {
			result.Error = err.Error()
		}
		results = append(results, result)
	}

	return connect.NewResponse(&flowwatchv1.BulkCancelResponse{
		Results: results,
	}), nil
}
