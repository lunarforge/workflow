package gateway

import (
	"context"
	"sync"

	"github.com/rs/zerolog/log"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
	"github.com/lunarforge/workflow/flowwatch/internal/adapter"
)

// fanOut runs fn against all connected (non-stale) subsystems in parallel and
// collects results. Errors from individual subsystems are logged but do not
// fail the overall operation — partial results are acceptable.
func fanOut[T any](ctx context.Context, ga *Adapter, fn func(ctx context.Context, conn *subsystemConn) ([]T, error)) []T {
	ga.mu.RLock()
	conns := make([]*subsystemConn, 0, len(ga.subsystems))
	for _, c := range ga.subsystems {
		if c.state() == connStateConnected {
			conns = append(conns, c)
		}
	}
	ga.mu.RUnlock()

	if len(conns) == 0 {
		return nil
	}

	type result struct {
		items []T
		err   error
		subID string
	}

	results := make(chan result, len(conns))
	var wg sync.WaitGroup
	for _, c := range conns {
		wg.Add(1)
		go func(c *subsystemConn) {
			defer wg.Done()
			items, err := fn(ctx, c)
			results <- result{items: items, err: err, subID: c.info.SubsystemID}
		}(c)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var merged []T
	for r := range results {
		if r.err != nil {
			log.Warn().Err(r.err).Str("subsystem", r.subID).Msg("gateway.fanout_error")
			continue
		}
		merged = append(merged, r.items...)
	}
	return merged
}

// fanOutSingle runs fn against all connected subsystems and returns the first
// non-error result. Used for broadcast lookups (e.g., find which subsystem owns
// a run ID).
func fanOutSingle[T any](ctx context.Context, ga *Adapter, fn func(ctx context.Context, conn *subsystemConn) (T, error)) (T, error) {
	ga.mu.RLock()
	conns := make([]*subsystemConn, 0, len(ga.subsystems))
	for _, c := range ga.subsystems {
		if c.state() == connStateConnected {
			conns = append(conns, c)
		}
	}
	ga.mu.RUnlock()

	var zero T
	if len(conns) == 0 {
		return zero, errNoSubsystems
	}

	type result struct {
		val   T
		err   error
		subID string
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan result, len(conns))
	for _, c := range conns {
		go func(c *subsystemConn) {
			val, err := fn(ctx, c)
			results <- result{val: val, err: err, subID: c.info.SubsystemID}
		}(c)
	}

	var lastErr error
	for range len(conns) {
		r := <-results
		if r.err == nil {
			cancel()
			return r.val, nil
		}
		lastErr = r.err
	}

	if lastErr != nil {
		return zero, lastErr
	}
	return zero, errNotFound
}

// routeByRunID looks up the owning subsystem for a run ID and routes the
// request. On cache miss, broadcasts to all subsystems.
func routeByRunID[T any](ctx context.Context, ga *Adapter, runID string, fn func(ctx context.Context, conn *subsystemConn) (T, error)) (T, error) {
	// Try cache first.
	if subID, ok := ga.router.Lookup(runID); ok {
		ga.mu.RLock()
		conn, exists := ga.subsystems[subID]
		ga.mu.RUnlock()
		if exists && conn.state() == connStateConnected {
			return fn(ctx, conn)
		}
	}

	// Cache miss — broadcast.
	return fanOutSingle(ctx, ga, fn)
}

// routeByWorkflowID routes to the subsystem that registered the given workflow.
func routeByWorkflowID[T any](ctx context.Context, ga *Adapter, workflowID string, fn func(ctx context.Context, conn *subsystemConn) (T, error)) (T, error) {
	ga.mu.RLock()
	subID, ok := ga.workflowIndex[workflowID]
	ga.mu.RUnlock()

	var zero T
	if !ok {
		return zero, errNotFound
	}

	ga.mu.RLock()
	conn, exists := ga.subsystems[subID]
	ga.mu.RUnlock()
	if !exists || conn.state() != connStateConnected {
		return zero, errSubsystemUnavailable
	}

	return fn(ctx, conn)
}

// --- Proto → Domain conversions ---
// These convert Connect-Go proto responses back into adapter domain types.

func runFromProto(pb *flowwatchv1.Run) adapter.RunData {
	if pb == nil {
		return adapter.RunData{}
	}

	r := adapter.RunData{
		ID:          pb.Id,
		WorkflowID:  pb.WorkflowId,
		ForeignID:   pb.ForeignId,
		SubsystemID: pb.SubsystemId,
		Status:      runStatusFromProto(pb.Status),
		CurrentStep: pb.CurrentStep,
		Metadata:    pb.Metadata,
	}

	if pb.StartedAt != nil {
		r.StartedAt = pb.StartedAt.AsTime()
	}
	if pb.FinishedAt != nil {
		t := pb.FinishedAt.AsTime()
		r.FinishedAt = &t
	}
	if pb.Duration != nil {
		r.Duration = pb.Duration.AsDuration()
	}
	if pb.Trace != nil {
		r.Trace = adapter.TraceContextData{
			TraceID:      pb.Trace.TraceId,
			SpanID:       pb.Trace.SpanId,
			ParentSpanID: pb.Trace.ParentSpanId,
			TraceSource:  pb.Trace.TraceSource,
		}
		r.Trace.AdditionalTraceIDs = pb.Trace.AdditionalTraceIds
	}
	if pb.Error != nil {
		r.Error = errorFromProto(pb.Error)
	}

	return r
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

func errorFromProto(pb *flowwatchv1.ErrorDetail) *adapter.ErrorData {
	if pb == nil {
		return nil
	}
	return &adapter.ErrorData{
		Message:    pb.Message,
		Code:       pb.Code,
		StackTrace: pb.StackTrace,
	}
}

func stepFromProto(pb *flowwatchv1.StepExecution) adapter.StepData {
	if pb == nil {
		return adapter.StepData{}
	}

	s := adapter.StepData{
		ID:       pb.Id,
		RunID:    pb.RunId,
		StepName: pb.StepName,
		Status:   stepStatusFromProto(pb.Status),
		Attempt:  int(pb.Attempt),
	}

	if pb.StartedAt != nil {
		s.StartedAt = pb.StartedAt.AsTime()
	}
	if pb.FinishedAt != nil {
		t := pb.FinishedAt.AsTime()
		s.FinishedAt = &t
	}
	if pb.Duration != nil {
		s.Duration = pb.Duration.AsDuration()
	}
	if pb.QueueWait != nil {
		s.QueueWait = pb.QueueWait.AsDuration()
	}
	if pb.Error != nil {
		s.Error = errorFromProto(pb.Error)
	}

	return s
}

func stepStatusFromProto(s flowwatchv1.StepStatus) string {
	switch s {
	case flowwatchv1.StepStatus_STEP_STATUS_PENDING:
		return "pending"
	case flowwatchv1.StepStatus_STEP_STATUS_RUNNING:
		return "running"
	case flowwatchv1.StepStatus_STEP_STATUS_SUCCEEDED:
		return "succeeded"
	case flowwatchv1.StepStatus_STEP_STATUS_FAILED:
		return "failed"
	case flowwatchv1.StepStatus_STEP_STATUS_SKIPPED:
		return "skipped"
	case flowwatchv1.StepStatus_STEP_STATUS_CANCELED:
		return "canceled"
	default:
		return ""
	}
}

func workflowDefFromProto(pb *flowwatchv1.WorkflowDefinition) adapter.WorkflowDef {
	if pb == nil {
		return adapter.WorkflowDef{}
	}

	def := adapter.WorkflowDef{
		ID:          pb.Id,
		Name:        pb.Name,
		Engine:      pb.Engine,
		Version:     pb.Version,
		SubsystemID: pb.SubsystemId,
		Metadata:    pb.Metadata,
	}
	if pb.CreatedAt != nil {
		def.CreatedAt = pb.CreatedAt.AsTime()
	}
	return def
}

func graphFromProto(pb *flowwatchv1.Graph) *adapter.GraphData {
	if pb == nil {
		return nil
	}

	g := &adapter.GraphData{WorkflowID: pb.WorkflowId}
	for _, n := range pb.Nodes {
		g.Nodes = append(g.Nodes, adapter.GraphNode{
			ID:       n.Id,
			Label:    n.Label,
			Type:     graphNodeTypeFromProto(n.Type),
			Metadata: n.Metadata,
		})
	}
	for _, e := range pb.Edges {
		g.Edges = append(g.Edges, adapter.GraphEdge{
			SourceID: e.SourceId,
			TargetID: e.TargetId,
			Label:    e.Label,
		})
	}
	return g
}

func graphNodeTypeFromProto(t flowwatchv1.GraphNodeType) string {
	switch t {
	case flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_STEP:
		return "step"
	case flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_CONDITION:
		return "condition"
	case flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_FORK:
		return "fork"
	case flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_JOIN:
		return "join"
	case flowwatchv1.GraphNodeType_GRAPH_NODE_TYPE_SUBWORKFLOW:
		return "subworkflow"
	default:
		return "step"
	}
}

func subsystemSummaryFromProto(pb *flowwatchv1.SubsystemSummary) adapter.SubsystemSummary {
	if pb == nil {
		return adapter.SubsystemSummary{}
	}

	s := adapter.SubsystemSummary{
		WorkflowCount:   int(pb.WorkflowCount),
		ActiveRuns:      int(pb.ActiveRuns),
		FailedLastHour:  int(pb.FailedLastHour),
		ErrorRateLastHr: pb.ErrorRateLastHour,
	}

	if pb.Subsystem != nil {
		s.Subsystem = adapter.SubsystemData{
			ID:          pb.Subsystem.Id,
			Name:        pb.Subsystem.Name,
			Description: pb.Subsystem.Description,
			Metadata:    pb.Subsystem.Metadata,
		}
		if pb.Subsystem.CreatedAt != nil {
			s.Subsystem.CreatedAt = pb.Subsystem.CreatedAt.AsTime()
		}
	}

	return s
}

func timelineFromProto(pb *flowwatchv1.GetRunTimelineResponse) *adapter.TimelineData {
	if pb == nil {
		return nil
	}

	t := &adapter.TimelineData{RunID: pb.RunId}
	if pb.TotalDuration != nil {
		t.TotalDuration = pb.TotalDuration.AsDuration()
	}
	for _, e := range pb.Entries {
		entry := adapter.TimelineEntry{
			StepName: e.StepName,
			Status:   stepStatusFromProto(e.Status),
			Attempt:  int(e.Attempt),
		}
		if e.Offset != nil {
			entry.Offset = e.Offset.AsDuration()
		}
		if e.Duration != nil {
			entry.Duration = e.Duration.AsDuration()
		}
		if e.QueueWait != nil {
			entry.QueueWait = e.QueueWait.AsDuration()
		}
		entry.Error = errorFromProto(e.Error)
		t.Entries = append(t.Entries, entry)
	}
	return t
}

func logFromProto(pb *flowwatchv1.LogEntry) adapter.LogData {
	if pb == nil {
		return adapter.LogData{}
	}

	l := adapter.LogData{
		RunID:      pb.RunId,
		StepName:   pb.StepName,
		LineNumber: pb.LineNumber,
		Message:    pb.Message,
		Source:     pb.Source,
	}
	if pb.Timestamp != nil {
		l.Timestamp = pb.Timestamp.AsTime()
	}
	l.Level = logLevelFromProto(pb.Level)
	return l
}

func logLevelFromProto(l flowwatchv1.LogLevel) string {
	switch l {
	case flowwatchv1.LogLevel_LOG_LEVEL_DEBUG:
		return "debug"
	case flowwatchv1.LogLevel_LOG_LEVEL_INFO:
		return "info"
	case flowwatchv1.LogLevel_LOG_LEVEL_WARN:
		return "warn"
	case flowwatchv1.LogLevel_LOG_LEVEL_ERROR:
		return "error"
	default:
		return "info"
	}
}

func capabilitiesFromProto(pb *flowwatchv1.Capabilities) *adapter.EngineCapabilities {
	if pb == nil {
		return &adapter.EngineCapabilities{}
	}
	return &adapter.EngineCapabilities{
		RetryRun:      pb.RetryRun,
		RetryFromStep: pb.RetryFromStep,
		CancelRun:     pb.CancelRun,
		PauseRun:      pb.PauseRun,
		ResumeRun:     pb.ResumeRun,
		SkipStep:      pb.SkipStep,
		Streaming:     pb.Streaming,
		Analytics:     pb.Analytics,
	}
}

// --- Domain → Proto conversions (for building proxy requests) ---

func runFilterToProto(f adapter.RunFilter) *flowwatchv1.RunFilter {
	pf := &flowwatchv1.RunFilter{
		WorkflowIds:   f.WorkflowIDs,
		SubsystemIds:  f.SubsystemIDs,
		ForeignId:     f.ForeignID,
		TraceId:       f.TraceID,
		MetadataMatch: f.MetadataMatch,
	}

	for _, s := range f.Statuses {
		pf.Statuses = append(pf.Statuses, runStatusToProto(s))
	}

	if f.From != nil || f.To != nil {
		pf.TimeRange = &flowwatchv1.TimeRange{}
		if f.From != nil {
			pf.TimeRange.From = timestamppb.New(*f.From)
		}
		if f.To != nil {
			pf.TimeRange.To = timestamppb.New(*f.To)
		}
	}

	return pf
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

func throughputFromProto(pb *flowwatchv1.ThroughputBucket) adapter.ThroughputPoint {
	p := adapter.ThroughputPoint{
		Succeeded: int(pb.Succeeded),
		Failed:    int(pb.Failed),
		Running:   int(pb.Running),
		Canceled:  int(pb.Canceled),
		Total:     int(pb.Total),
	}
	if pb.Timestamp != nil {
		p.Timestamp = pb.Timestamp.AsTime()
	}
	return p
}

func latencyFromProto(pb *flowwatchv1.LatencyBucket) adapter.LatencyPoint {
	p := adapter.LatencyPoint{}
	if pb.Timestamp != nil {
		p.Timestamp = pb.Timestamp.AsTime()
	}
	if pb.P50 != nil {
		p.P50 = pb.P50.AsDuration()
	}
	if pb.P95 != nil {
		p.P95 = pb.P95.AsDuration()
	}
	if pb.P99 != nil {
		p.P99 = pb.P99.AsDuration()
	}
	return p
}

func failureRateFromProto(pb *flowwatchv1.FailureRateBucket) adapter.FailureRatePoint {
	p := adapter.FailureRatePoint{
		Rate:       pb.Rate,
		TotalRuns:  int(pb.TotalRuns),
		FailedRuns: int(pb.FailedRuns),
	}
	if pb.Timestamp != nil {
		p.Timestamp = pb.Timestamp.AsTime()
	}
	return p
}

func heatmapFromProto(pb *flowwatchv1.HeatmapCell) adapter.HeatmapPoint {
	p := adapter.HeatmapPoint{
		StepName:        pb.StepName,
		FailureRate:     pb.FailureRate,
		TotalExecutions: int(pb.TotalExecutions),
		FailedExecs:     int(pb.FailedExecutions),
	}
	if pb.BucketStart != nil {
		p.BucketStart = pb.BucketStart.AsTime()
	}
	if pb.AvgDuration != nil {
		p.AvgDuration = pb.AvgDuration.AsDuration()
	}
	return p
}

func stepDurationFromProto(pb *flowwatchv1.GetStepDurationResponse) *adapter.StepDurationReport {
	if pb == nil {
		return nil
	}

	r := &adapter.StepDurationReport{}
	if pb.BottleneckThreshold != nil {
		r.BottleneckThreshold = pb.BottleneckThreshold.AsDuration()
	}

	for _, s := range pb.Steps {
		entry := adapter.StepDurationEntry{
			StepName:     s.StepName,
			IsBottleneck: s.IsBottleneck,
			SampleCount:  s.SampleCount,
		}
		entry.QueueWait = percentilesFromProto(s.QueueWait)
		entry.Execution = percentilesFromProto(s.Execution)
		entry.Total = percentilesFromProto(s.Total)

		for _, t := range s.Trend {
			tp := adapter.StepTrendPoint{}
			if t.Timestamp != nil {
				tp.Timestamp = t.Timestamp.AsTime()
			}
			if t.P50 != nil {
				tp.P50 = t.P50.AsDuration()
			}
			if t.P95 != nil {
				tp.P95 = t.P95.AsDuration()
			}
			if t.P99 != nil {
				tp.P99 = t.P99.AsDuration()
			}
			entry.Trend = append(entry.Trend, tp)
		}

		r.Steps = append(r.Steps, entry)
	}

	return r
}

func percentilesFromProto(pb *flowwatchv1.DurationPercentiles) adapter.DurationPercentiles {
	if pb == nil {
		return adapter.DurationPercentiles{}
	}
	var d adapter.DurationPercentiles
	if pb.P50 != nil {
		d.P50 = pb.P50.AsDuration()
	}
	if pb.P95 != nil {
		d.P95 = pb.P95.AsDuration()
	}
	if pb.P99 != nil {
		d.P99 = pb.P99.AsDuration()
	}
	return d
}
