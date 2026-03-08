package server

import (
	"context"
	"sort"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowwatchv1 "github.com/luno/workflow/flowwatch/gen/flowwatch/v1"
	"github.com/luno/workflow/flowwatch/internal/adapter"
)

func analyticsParamsFromProto(f *flowwatchv1.AnalyticsFilter) adapter.AnalyticsParams {
	p := adapter.AnalyticsParams{
		Granularity: "1h",
	}
	if f == nil {
		p.From = time.Now().Add(-24 * time.Hour)
		p.To = time.Now()
		return p
	}
	p.WorkflowID = f.WorkflowId
	if f.Granularity != "" {
		p.Granularity = f.Granularity
	}
	if f.TimeRange != nil {
		if f.TimeRange.From != nil {
			p.From = f.TimeRange.From.AsTime()
		}
		if f.TimeRange.To != nil {
			p.To = f.TimeRange.To.AsTime()
		}
	}
	if p.From.IsZero() {
		p.From = time.Now().Add(-24 * time.Hour)
	}
	if p.To.IsZero() {
		p.To = time.Now()
	}
	return p
}

func (s *analyticsServer) GetThroughput(ctx context.Context, req *connect.Request[flowwatchv1.GetThroughputRequest]) (*connect.Response[flowwatchv1.GetThroughputResponse], error) {
	params := analyticsParamsFromProto(req.Msg.Filter)

	points, err := s.adapter.GetThroughput(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var buckets []*flowwatchv1.ThroughputBucket
	for _, pt := range points {
		buckets = append(buckets, &flowwatchv1.ThroughputBucket{
			Timestamp: timestamppb.New(pt.Timestamp),
			Succeeded: int32(pt.Succeeded),
			Failed:    int32(pt.Failed),
			Running:   int32(pt.Running),
			Canceled:  int32(pt.Canceled),
			Total:     int32(pt.Total),
		})
	}

	return connect.NewResponse(&flowwatchv1.GetThroughputResponse{
		Buckets: buckets,
	}), nil
}

func (s *analyticsServer) GetLatency(ctx context.Context, req *connect.Request[flowwatchv1.GetLatencyRequest]) (*connect.Response[flowwatchv1.GetLatencyResponse], error) {
	params := analyticsParamsFromProto(req.Msg.Filter)

	points, err := s.adapter.GetLatency(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var buckets []*flowwatchv1.LatencyBucket
	for _, pt := range points {
		buckets = append(buckets, &flowwatchv1.LatencyBucket{
			Timestamp: timestamppb.New(pt.Timestamp),
			P50:       durationpb.New(pt.P50),
			P95:       durationpb.New(pt.P95),
			P99:       durationpb.New(pt.P99),
		})
	}

	return connect.NewResponse(&flowwatchv1.GetLatencyResponse{
		Buckets: buckets,
	}), nil
}

func (s *analyticsServer) GetFailureRate(ctx context.Context, req *connect.Request[flowwatchv1.GetFailureRateRequest]) (*connect.Response[flowwatchv1.GetFailureRateResponse], error) {
	params := analyticsParamsFromProto(req.Msg.Filter)

	points, err := s.adapter.GetFailureRate(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var buckets []*flowwatchv1.FailureRateBucket
	for _, pt := range points {
		buckets = append(buckets, &flowwatchv1.FailureRateBucket{
			Timestamp:  timestamppb.New(pt.Timestamp),
			Rate:       pt.Rate,
			TotalRuns:  int32(pt.TotalRuns),
			FailedRuns: int32(pt.FailedRuns),
		})
	}

	return connect.NewResponse(&flowwatchv1.GetFailureRateResponse{
		Buckets: buckets,
	}), nil
}

func (s *analyticsServer) GetStepDuration(ctx context.Context, req *connect.Request[flowwatchv1.GetStepDurationRequest]) (*connect.Response[flowwatchv1.GetStepDurationResponse], error) {
	params := adapter.StepDurationParams{
		WorkflowID: req.Msg.WorkflowId,
	}
	if req.Msg.TimeRange != nil {
		if req.Msg.TimeRange.From != nil {
			params.From = req.Msg.TimeRange.From.AsTime()
		}
		if req.Msg.TimeRange.To != nil {
			params.To = req.Msg.TimeRange.To.AsTime()
		}
	}
	if params.From.IsZero() {
		params.From = time.Now().Add(-7 * 24 * time.Hour)
	}
	if params.To.IsZero() {
		params.To = time.Now()
	}

	report, err := s.adapter.GetStepDuration(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	var steps []*flowwatchv1.StepDurationEntry
	for _, e := range report.Steps {
		var trend []*flowwatchv1.StepDurationTrendPoint
		for _, tp := range e.Trend {
			trend = append(trend, &flowwatchv1.StepDurationTrendPoint{
				Timestamp: timestamppb.New(tp.Timestamp),
				P50:       durationpb.New(tp.P50),
				P95:       durationpb.New(tp.P95),
				P99:       durationpb.New(tp.P99),
			})
		}

		steps = append(steps, &flowwatchv1.StepDurationEntry{
			StepName:     e.StepName,
			IsBottleneck: e.IsBottleneck,
			QueueWait:    durationPercentilesToProto(e.QueueWait),
			Execution:    durationPercentilesToProto(e.Execution),
			Total:        durationPercentilesToProto(e.Total),
			SampleCount:  e.SampleCount,
			Trend:        trend,
		})
	}

	return connect.NewResponse(&flowwatchv1.GetStepDurationResponse{
		Steps:               steps,
		BottleneckThreshold: durationpb.New(report.BottleneckThreshold),
	}), nil
}

func (s *analyticsServer) GetStepHeatmap(ctx context.Context, req *connect.Request[flowwatchv1.GetStepHeatmapRequest]) (*connect.Response[flowwatchv1.GetStepHeatmapResponse], error) {
	params := analyticsParamsFromProto(req.Msg.Filter)

	points, err := s.adapter.GetStepHeatmap(ctx, params)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	// Collect unique step names and bucket timestamps for the response.
	stepSet := make(map[string]bool)
	bucketSet := make(map[time.Time]bool)

	var cells []*flowwatchv1.HeatmapCell
	for _, pt := range points {
		stepSet[pt.StepName] = true
		bucketSet[pt.BucketStart] = true
		cells = append(cells, &flowwatchv1.HeatmapCell{
			StepName:         pt.StepName,
			BucketStart:      timestamppb.New(pt.BucketStart),
			FailureRate:      pt.FailureRate,
			TotalExecutions:  int32(pt.TotalExecutions),
			FailedExecutions: int32(pt.FailedExecs),
			AvgDuration:      durationpb.New(pt.AvgDuration),
		})
	}

	// Build ordered step names.
	var stepNames []string
	for name := range stepSet {
		stepNames = append(stepNames, name)
	}
	sort.Strings(stepNames)

	// Build ordered bucket timestamps.
	var bucketTimes []time.Time
	for t := range bucketSet {
		bucketTimes = append(bucketTimes, t)
	}
	sort.Slice(bucketTimes, func(i, j int) bool { return bucketTimes[i].Before(bucketTimes[j]) })

	var bucketTimestamps []*timestamppb.Timestamp
	for _, t := range bucketTimes {
		bucketTimestamps = append(bucketTimestamps, timestamppb.New(t))
	}

	return connect.NewResponse(&flowwatchv1.GetStepHeatmapResponse{
		Cells:            cells,
		StepNames:        stepNames,
		BucketTimestamps: bucketTimestamps,
	}), nil
}

func durationPercentilesToProto(dp adapter.DurationPercentiles) *flowwatchv1.DurationPercentiles {
	return &flowwatchv1.DurationPercentiles{
		P50: durationpb.New(dp.P50),
		P95: durationpb.New(dp.P95),
		P99: durationpb.New(dp.P99),
	}
}
