// Package gateway provides a central FlowWatch gateway that aggregates
// multiple subsystem FlowWatch instances behind a single EngineAdapter.
// Subsystems register via a persistent bidirectional gRPC stream, and the
// gateway proxies UI requests to the appropriate subsystem.
package gateway

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
	"github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1/flowwatchv1connect"
	"github.com/lunarforge/workflow/flowwatch/internal/adapter"
)

var (
	errNoSubsystems        = errors.New("no subsystems connected")
	errNotFound            = errors.New("not found")
	errSubsystemUnavailable = errors.New("subsystem unavailable")
)

// connState represents the connection state of a subsystem.
type connState int

const (
	connStateConnected connState = iota
	connStateStale
)

// SubsystemInfo holds metadata about a registered subsystem.
type SubsystemInfo struct {
	SubsystemID  string
	SubsystemName string
	Description  string
	Endpoint     string
	Workflows    []*flowwatchv1.WorkflowDefinition
	Capabilities *flowwatchv1.Capabilities
	Metadata     map[string]string
}

// subsystemConn represents a connected subsystem with its Connect-Go clients.
type subsystemConn struct {
	info          SubsystemInfo
	mu            sync.RWMutex
	connState     connState
	lastHeartbeat time.Time
	staleSince    *time.Time

	// Connect-Go clients for proxying.
	workflow  flowwatchv1connect.WorkflowServiceClient
	run       flowwatchv1connect.RunServiceClient
	analytics flowwatchv1connect.AnalyticsServiceClient
	stream    flowwatchv1connect.StreamServiceClient
}

func (c *subsystemConn) state() connState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connState
}

func (c *subsystemConn) markStale() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connState = connStateStale
	now := time.Now()
	c.staleSince = &now
}

func (c *subsystemConn) markConnected() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connState = connStateConnected
	c.staleSince = nil
	c.lastHeartbeat = time.Now()
}

func (c *subsystemConn) recordHeartbeat() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastHeartbeat = time.Now()
}

func newSubsystemConn(info SubsystemInfo, httpClient *http.Client) *subsystemConn {
	return &subsystemConn{
		info:          info,
		connState:     connStateConnected,
		lastHeartbeat: time.Now(),
		workflow:      flowwatchv1connect.NewWorkflowServiceClient(httpClient, info.Endpoint),
		run:           flowwatchv1connect.NewRunServiceClient(httpClient, info.Endpoint),
		analytics:     flowwatchv1connect.NewAnalyticsServiceClient(httpClient, info.Endpoint),
		stream:        flowwatchv1connect.NewStreamServiceClient(httpClient, info.Endpoint),
	}
}

// Adapter implements adapter.EngineAdapter by routing requests to registered
// subsystem FlowWatch instances.
type Adapter struct {
	mu            sync.RWMutex
	subsystems    map[string]*subsystemConn // subsystem ID -> conn
	workflowIndex map[string]string         // workflow ID -> subsystem ID
	router        *runRouter
	config        adapterConfig
}

// NewAdapter creates a new gateway adapter.
func NewAdapter(opts ...AdapterOption) *Adapter {
	cfg := defaultAdapterConfig()
	for _, opt := range opts {
		opt(&cfg)
	}

	return &Adapter{
		subsystems:    make(map[string]*subsystemConn),
		workflowIndex: make(map[string]string),
		router:        newRunRouter(cfg.runCacheSize),
		config:        cfg,
	}
}

// RegisterSubsystem adds a subsystem to the gateway registry.
func (a *Adapter) RegisterSubsystem(info SubsystemInfo) {
	conn := newSubsystemConn(info, a.config.httpClient)

	a.mu.Lock()
	defer a.mu.Unlock()

	a.subsystems[info.SubsystemID] = conn
	for _, wf := range info.Workflows {
		a.workflowIndex[wf.Id] = info.SubsystemID
	}
}

// DeregisterSubsystem removes a subsystem from the registry.
func (a *Adapter) DeregisterSubsystem(subsystemID string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Remove workflow index entries.
	for wfID, subID := range a.workflowIndex {
		if subID == subsystemID {
			delete(a.workflowIndex, wfID)
		}
	}
	delete(a.subsystems, subsystemID)
	a.router.RemoveSubsystem(subsystemID)
}

// MarkStale marks a subsystem as stale (disconnected but within grace period).
func (a *Adapter) MarkStale(subsystemID string) {
	a.mu.RLock()
	conn, ok := a.subsystems[subsystemID]
	a.mu.RUnlock()
	if ok {
		conn.markStale()
	}
}

// --- EngineAdapter implementation ---

func (a *Adapter) EngineName() string {
	return "flowwatch-gateway"
}

func (a *Adapter) Capabilities() *adapter.EngineCapabilities {
	// Union of all connected subsystems' capabilities.
	a.mu.RLock()
	defer a.mu.RUnlock()

	caps := &adapter.EngineCapabilities{}
	for _, c := range a.subsystems {
		if c.state() != connStateConnected {
			continue
		}
		if c.info.Capabilities == nil {
			continue
		}
		pb := c.info.Capabilities
		caps.RetryRun = caps.RetryRun || pb.RetryRun
		caps.RetryFromStep = caps.RetryFromStep || pb.RetryFromStep
		caps.CancelRun = caps.CancelRun || pb.CancelRun
		caps.PauseRun = caps.PauseRun || pb.PauseRun
		caps.ResumeRun = caps.ResumeRun || pb.ResumeRun
		caps.SkipStep = caps.SkipStep || pb.SkipStep
		caps.Streaming = caps.Streaming || pb.Streaming
		caps.Analytics = caps.Analytics || pb.Analytics
	}
	return caps
}

// --- Subsystems ---

func (a *Adapter) ListSubsystems(_ context.Context) ([]adapter.SubsystemSummary, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []adapter.SubsystemSummary
	for _, c := range a.subsystems {
		wfCount := 0
		for _, subID := range a.workflowIndex {
			if subID == c.info.SubsystemID {
				wfCount++
			}
		}
		result = append(result, adapter.SubsystemSummary{
			Subsystem: adapter.SubsystemData{
				ID:          c.info.SubsystemID,
				Name:        c.info.SubsystemName,
				Description: c.info.Description,
				Metadata:    c.info.Metadata,
			},
			WorkflowCount: wfCount,
		})
	}
	return result, nil
}

func (a *Adapter) GetSubsystem(ctx context.Context, subsystemID string) (*adapter.SubsystemDetail, error) {
	a.mu.RLock()
	conn, ok := a.subsystems[subsystemID]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("subsystem not found: %s", subsystemID)
	}

	resp, err := conn.workflow.GetSubsystem(ctx, connect.NewRequest(&flowwatchv1.GetSubsystemRequest{
		SubsystemId: subsystemID,
	}))
	if err != nil {
		return nil, err
	}

	detail := &adapter.SubsystemDetail{
		Summary: subsystemSummaryFromProto(resp.Msg.Subsystem),
	}
	for _, wf := range resp.Msg.Workflows {
		detail.Workflows = append(detail.Workflows, workflowDefFromProto(wf))
	}
	return detail, nil
}

// --- Workflows ---

func (a *Adapter) ListWorkflows(ctx context.Context) ([]adapter.WorkflowDef, error) {
	results := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.WorkflowDef, error) {
		resp, err := conn.workflow.ListWorkflows(ctx, connect.NewRequest(&flowwatchv1.ListWorkflowsRequest{}))
		if err != nil {
			return nil, err
		}
		var defs []adapter.WorkflowDef
		for _, wf := range resp.Msg.Workflows {
			defs = append(defs, workflowDefFromProto(wf))
		}
		return defs, nil
	})
	return results, nil
}

func (a *Adapter) GetWorkflow(ctx context.Context, workflowID string) (*adapter.WorkflowDef, error) {
	return routeByWorkflowID(ctx, a, workflowID, func(ctx context.Context, conn *subsystemConn) (*adapter.WorkflowDef, error) {
		resp, err := conn.workflow.GetWorkflow(ctx, connect.NewRequest(&flowwatchv1.GetWorkflowRequest{
			WorkflowId: workflowID,
		}))
		if err != nil {
			return nil, err
		}
		def := workflowDefFromProto(resp.Msg.Workflow)
		return &def, nil
	})
}

func (a *Adapter) GetGraph(ctx context.Context, workflowID string) (*adapter.GraphData, error) {
	return routeByWorkflowID(ctx, a, workflowID, func(ctx context.Context, conn *subsystemConn) (*adapter.GraphData, error) {
		resp, err := conn.workflow.GetGraph(ctx, connect.NewRequest(&flowwatchv1.GetGraphRequest{
			WorkflowId: workflowID,
		}))
		if err != nil {
			return nil, err
		}
		return graphFromProto(resp.Msg.Graph), nil
	})
}

// --- Traces ---

func (a *Adapter) GetTrace(ctx context.Context, traceID string) (*adapter.TraceData, error) {
	// Fan-out: traces can span subsystems.
	allSpans := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.TraceSpan, error) {
		resp, err := conn.workflow.GetTrace(ctx, connect.NewRequest(&flowwatchv1.GetTraceRequest{
			TraceId: traceID,
		}))
		if err != nil {
			return nil, err
		}
		var spans []adapter.TraceSpan
		for _, s := range resp.Msg.Spans {
			span := adapter.TraceSpan{
				SpanID:       s.SpanId,
				ParentSpanID: s.ParentSpanId,
				Run:          runFromProto(s.Run),
				SubsystemID:  s.SubsystemId,
			}
			if s.Offset != nil {
				span.Offset = s.Offset.AsDuration()
			}
			if s.Duration != nil {
				span.Duration = s.Duration.AsDuration()
			}
			for _, step := range s.Steps {
				span.Steps = append(span.Steps, stepFromProto(step))
			}
			spans = append(spans, span)
		}
		return spans, nil
	})

	if len(allSpans) == 0 {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	// Count unique subsystems.
	subs := make(map[string]struct{})
	for _, s := range allSpans {
		subs[s.SubsystemID] = struct{}{}
	}

	return &adapter.TraceData{
		TraceID:        traceID,
		Spans:          allSpans,
		SubsystemCount: len(subs),
	}, nil
}

// --- Runs ---

func (a *Adapter) ListRuns(ctx context.Context, filter adapter.RunFilter, cursor string, limit int) ([]adapter.RunData, string, error) {
	pf := runFilterToProto(filter)

	results := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.RunData, error) {
		resp, err := conn.run.ListRuns(ctx, connect.NewRequest(&flowwatchv1.ListRunsRequest{
			Filter: pf,
			Pagination: &flowwatchv1.PaginationRequest{
				Cursor: cursor,
				Limit:  int32(limit),
			},
		}))
		if err != nil {
			return nil, err
		}
		var runs []adapter.RunData
		for _, r := range resp.Msg.Runs {
			run := runFromProto(r)
			runs = append(runs, run)
			a.router.Record(run.ID, run.SubsystemID)
		}
		return runs, nil
	})

	// Trim to limit.
	if len(results) > limit && limit > 0 {
		results = results[:limit]
	}

	return results, "", nil
}

func (a *Adapter) GetRun(ctx context.Context, runID string) (*adapter.RunData, error) {
	return routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (*adapter.RunData, error) {
		resp, err := conn.run.GetRun(ctx, connect.NewRequest(&flowwatchv1.GetRunRequest{
			RunId: runID,
		}))
		if err != nil {
			return nil, err
		}
		run := runFromProto(resp.Msg.Run)
		a.router.Record(run.ID, run.SubsystemID)
		return &run, nil
	})
}

func (a *Adapter) GetRunSteps(ctx context.Context, runID string) ([]adapter.StepData, error) {
	steps, err := routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) ([]adapter.StepData, error) {
		resp, err := conn.run.GetRunSteps(ctx, connect.NewRequest(&flowwatchv1.GetRunStepsRequest{
			RunId: runID,
		}))
		if err != nil {
			return nil, err
		}
		var result []adapter.StepData
		for _, s := range resp.Msg.Steps {
			result = append(result, stepFromProto(s))
		}
		return result, nil
	})
	return steps, err
}

func (a *Adapter) GetRunTimeline(ctx context.Context, runID string) (*adapter.TimelineData, error) {
	return routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (*adapter.TimelineData, error) {
		resp, err := conn.run.GetRunTimeline(ctx, connect.NewRequest(&flowwatchv1.GetRunTimelineRequest{
			RunId: runID,
		}))
		if err != nil {
			return nil, err
		}
		return timelineFromProto(resp.Msg), nil
	})
}

func (a *Adapter) GetRunLogs(ctx context.Context, runID string, stepName string, cursor string, limit int) ([]adapter.LogData, string, error) {
	type logsResult struct {
		logs   []adapter.LogData
		cursor string
	}

	result, err := routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (logsResult, error) {
		resp, err := conn.run.GetRunLogs(ctx, connect.NewRequest(&flowwatchv1.GetRunLogsRequest{
			RunId:    runID,
			StepName: stepName,
			Pagination: &flowwatchv1.PaginationRequest{
				Cursor: cursor,
				Limit:  int32(limit),
			},
		}))
		if err != nil {
			return logsResult{}, err
		}
		var logs []adapter.LogData
		for _, l := range resp.Msg.Logs {
			logs = append(logs, logFromProto(l))
		}
		nextCursor := ""
		if resp.Msg.Pagination != nil {
			nextCursor = resp.Msg.Pagination.NextCursor
		}
		return logsResult{logs: logs, cursor: nextCursor}, nil
	})
	if err != nil {
		return nil, "", err
	}
	return result.logs, result.cursor, nil
}

// --- Actions ---

func (a *Adapter) RetryRun(ctx context.Context, runID string) (string, error) {
	return routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (string, error) {
		resp, err := conn.run.RetryRun(ctx, connect.NewRequest(&flowwatchv1.RetryRunRequest{
			RunId: runID,
		}))
		if err != nil {
			return "", err
		}
		return resp.Msg.NewRunId, nil
	})
}

func (a *Adapter) RetryFromStep(ctx context.Context, runID, stepName string) (string, error) {
	return routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (string, error) {
		resp, err := conn.run.RetryFromStep(ctx, connect.NewRequest(&flowwatchv1.RetryFromStepRequest{
			RunId:    runID,
			StepName: stepName,
		}))
		if err != nil {
			return "", err
		}
		return resp.Msg.NewRunId, nil
	})
}

func (a *Adapter) CancelRun(ctx context.Context, runID string, reason string) error {
	_, err := routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (struct{}, error) {
		_, err := conn.run.CancelRun(ctx, connect.NewRequest(&flowwatchv1.CancelRunRequest{
			RunId:  runID,
			Reason: reason,
		}))
		return struct{}{}, err
	})
	return err
}

func (a *Adapter) PauseRun(ctx context.Context, runID string) error {
	_, err := routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (struct{}, error) {
		_, err := conn.run.PauseRun(ctx, connect.NewRequest(&flowwatchv1.PauseRunRequest{
			RunId: runID,
		}))
		return struct{}{}, err
	})
	return err
}

func (a *Adapter) ResumeRun(ctx context.Context, runID string) error {
	_, err := routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (struct{}, error) {
		_, err := conn.run.ResumeRun(ctx, connect.NewRequest(&flowwatchv1.ResumeRunRequest{
			RunId: runID,
		}))
		return struct{}{}, err
	})
	return err
}

func (a *Adapter) SkipStep(ctx context.Context, runID, stepName string, reason string) error {
	_, err := routeByRunID(ctx, a, runID, func(ctx context.Context, conn *subsystemConn) (struct{}, error) {
		_, err := conn.run.SkipStep(ctx, connect.NewRequest(&flowwatchv1.SkipStepRequest{
			RunId:    runID,
			StepName: stepName,
			Reason:   reason,
		}))
		return struct{}{}, err
	})
	return err
}

// --- Streaming ---

func (a *Adapter) SubscribeRuns(_ context.Context, _ adapter.RunFilter) (<-chan adapter.RunEvent, error) {
	// TODO: fan-out to all subsystems and multiplex channels.
	return nil, errors.New("streaming not yet implemented in gateway")
}

func (a *Adapter) SubscribeRun(_ context.Context, _ string) (<-chan adapter.RunDetailEvent, error) {
	// TODO: route to owning subsystem and proxy the channel.
	return nil, errors.New("streaming not yet implemented in gateway")
}

// --- Analytics ---

func (a *Adapter) GetThroughput(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.ThroughputPoint, error) {
	filter := &flowwatchv1.AnalyticsFilter{
		WorkflowId:  p.WorkflowID,
		Granularity: p.Granularity,
		TimeRange: &flowwatchv1.TimeRange{
			From: timestamppb.New(p.From),
			To:   timestamppb.New(p.To),
		},
	}

	results := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.ThroughputPoint, error) {
		resp, err := conn.analytics.GetThroughput(ctx, connect.NewRequest(&flowwatchv1.GetThroughputRequest{
			Filter: filter,
		}))
		if err != nil {
			return nil, err
		}
		var points []adapter.ThroughputPoint
		for _, b := range resp.Msg.Buckets {
			points = append(points, throughputFromProto(b))
		}
		return points, nil
	})
	return results, nil
}

func (a *Adapter) GetLatency(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.LatencyPoint, error) {
	filter := &flowwatchv1.AnalyticsFilter{
		WorkflowId:  p.WorkflowID,
		Granularity: p.Granularity,
		TimeRange: &flowwatchv1.TimeRange{
			From: timestamppb.New(p.From),
			To:   timestamppb.New(p.To),
		},
	}

	results := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.LatencyPoint, error) {
		resp, err := conn.analytics.GetLatency(ctx, connect.NewRequest(&flowwatchv1.GetLatencyRequest{
			Filter: filter,
		}))
		if err != nil {
			return nil, err
		}
		var points []adapter.LatencyPoint
		for _, b := range resp.Msg.Buckets {
			points = append(points, latencyFromProto(b))
		}
		return points, nil
	})
	return results, nil
}

func (a *Adapter) GetFailureRate(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.FailureRatePoint, error) {
	filter := &flowwatchv1.AnalyticsFilter{
		WorkflowId:  p.WorkflowID,
		Granularity: p.Granularity,
		TimeRange: &flowwatchv1.TimeRange{
			From: timestamppb.New(p.From),
			To:   timestamppb.New(p.To),
		},
	}

	results := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.FailureRatePoint, error) {
		resp, err := conn.analytics.GetFailureRate(ctx, connect.NewRequest(&flowwatchv1.GetFailureRateRequest{
			Filter: filter,
		}))
		if err != nil {
			return nil, err
		}
		var points []adapter.FailureRatePoint
		for _, b := range resp.Msg.Buckets {
			points = append(points, failureRateFromProto(b))
		}
		return points, nil
	})
	return results, nil
}

func (a *Adapter) GetStepDuration(ctx context.Context, p adapter.StepDurationParams) (*adapter.StepDurationReport, error) {
	return fanOutSingle(ctx, a, func(ctx context.Context, conn *subsystemConn) (*adapter.StepDurationReport, error) {
		resp, err := conn.analytics.GetStepDuration(ctx, connect.NewRequest(&flowwatchv1.GetStepDurationRequest{
			WorkflowId: p.WorkflowID,
			TimeRange: &flowwatchv1.TimeRange{
				From: timestamppb.New(p.From),
				To:   timestamppb.New(p.To),
			},
		}))
		if err != nil {
			return nil, err
		}
		return stepDurationFromProto(resp.Msg), nil
	})
}

func (a *Adapter) GetStepHeatmap(ctx context.Context, p adapter.AnalyticsParams) ([]adapter.HeatmapPoint, error) {
	filter := &flowwatchv1.AnalyticsFilter{
		WorkflowId:  p.WorkflowID,
		Granularity: p.Granularity,
		TimeRange: &flowwatchv1.TimeRange{
			From: timestamppb.New(p.From),
			To:   timestamppb.New(p.To),
		},
	}

	results := fanOut(ctx, a, func(ctx context.Context, conn *subsystemConn) ([]adapter.HeatmapPoint, error) {
		resp, err := conn.analytics.GetStepHeatmap(ctx, connect.NewRequest(&flowwatchv1.GetStepHeatmapRequest{
			Filter: filter,
		}))
		if err != nil {
			return nil, err
		}
		var points []adapter.HeatmapPoint
		for _, c := range resp.Msg.Cells {
			points = append(points, heatmapFromProto(c))
		}
		return points, nil
	})
	return results, nil
}
