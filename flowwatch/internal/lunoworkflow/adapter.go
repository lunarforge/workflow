// Package lunoworkflow provides an EngineAdapter implementation for the luno/workflow library.
package lunoworkflow

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	"github.com/luno/workflow"

	"github.com/luno/workflow/flowwatch/internal/adapter"
)

// RetryFunc retries a finished workflow run, returning the new run ID.
type RetryFunc func(ctx context.Context, runID string) (string, error)

// WorkflowRegistration holds the metadata and action bindings for a registered workflow.
type WorkflowRegistration struct {
	Name      string
	GraphInfo workflow.GraphInfo
	Labels    map[int]string // status int -> human-readable label
	Subsystem string
	Metadata  map[string]string
	RetryFn   RetryFunc
}

// RegisterWorkflow is a generic helper that extracts graph info and status labels from a
// typed Workflow instance and registers it with the adapter.
func RegisterWorkflow[Type any, Status workflow.StatusType](a *Adapter, wf *workflow.Workflow[Type, Status]) {
	info := wf.GraphInfo()
	labels := make(map[int]string)

	for _, n := range info.Nodes {
		labels[n] = Status(n).String()
	}

	a.Register(WorkflowRegistration{
		Name:      wf.Name(),
		GraphInfo: info,
		Labels:    labels,
		RetryFn: func(ctx context.Context, runID string) (string, error) {
			return wf.Retry(ctx, runID)
		},
	})
}

// New creates a new Adapter backed by the given record store and optional step store.
func New(recordStore workflow.RecordStore, stepStore workflow.StepStore) *Adapter {
	return &Adapter{
		recordStore: recordStore,
		stepStore:   stepStore,
		workflows:   make(map[string]*workflowEntry),
	}
}

type workflowEntry struct {
	reg WorkflowRegistration
}

// Adapter implements adapter.EngineAdapter for luno/workflow.
type Adapter struct {
	mu          sync.RWMutex
	recordStore workflow.RecordStore
	stepStore   workflow.StepStore
	workflows   map[string]*workflowEntry
}

var _ adapter.EngineAdapter = (*Adapter)(nil)

// Register adds a workflow to the adapter's registry.
func (a *Adapter) Register(reg WorkflowRegistration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.workflows[reg.Name] = &workflowEntry{reg: reg}
}

func (a *Adapter) EngineName() string {
	return "luno/workflow"
}

func (a *Adapter) Capabilities() *adapter.EngineCapabilities {
	return &adapter.EngineCapabilities{
		RetryRun:  true,
		CancelRun: true,
		PauseRun:  true,
		ResumeRun: true,
		// Not supported by luno/workflow:
		RetryFromStep: false,
		SkipStep:      false,
		Streaming:     false,
		Analytics:     a.stepStore != nil,
	}
}

// --- Subsystems ---

func (a *Adapter) ListSubsystems(ctx context.Context) ([]adapter.SubsystemSummary, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	subs := make(map[string]*adapter.SubsystemSummary)
	for _, entry := range a.workflows {
		sub := entry.reg.Subsystem
		if sub == "" {
			sub = "default"
		}
		s, ok := subs[sub]
		if !ok {
			s = &adapter.SubsystemSummary{
				Subsystem: adapter.SubsystemData{
					ID:   sub,
					Name: sub,
				},
			}
			subs[sub] = s
		}
		s.WorkflowCount++
	}

	result := make([]adapter.SubsystemSummary, 0, len(subs))
	for _, s := range subs {
		result = append(result, *s)
	}
	return result, nil
}

func (a *Adapter) GetSubsystem(ctx context.Context, subsystemID string) (*adapter.SubsystemDetail, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var workflows []adapter.WorkflowDef
	for _, entry := range a.workflows {
		sub := entry.reg.Subsystem
		if sub == "" {
			sub = "default"
		}
		if sub != subsystemID {
			continue
		}
		workflows = append(workflows, toWorkflowDef(entry))
	}

	if len(workflows) == 0 {
		return nil, fmt.Errorf("subsystem not found: %s", subsystemID)
	}

	return &adapter.SubsystemDetail{
		Summary: adapter.SubsystemSummary{
			Subsystem: adapter.SubsystemData{
				ID:   subsystemID,
				Name: subsystemID,
			},
			WorkflowCount: len(workflows),
		},
		Workflows: workflows,
	}, nil
}

// --- Workflow definitions ---

func (a *Adapter) ListWorkflows(ctx context.Context) ([]adapter.WorkflowDef, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]adapter.WorkflowDef, 0, len(a.workflows))
	for _, entry := range a.workflows {
		result = append(result, toWorkflowDef(entry))
	}
	return result, nil
}

func (a *Adapter) GetWorkflow(ctx context.Context, workflowID string) (*adapter.WorkflowDef, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entry, ok := a.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}
	def := toWorkflowDef(entry)
	return &def, nil
}

func (a *Adapter) GetGraph(ctx context.Context, workflowID string) (*adapter.GraphData, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	entry, ok := a.workflows[workflowID]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	gi := entry.reg.GraphInfo
	var nodes []adapter.GraphNode
	for _, n := range gi.Nodes {
		label := entry.reg.Labels[n]
		if label == "" {
			label = strconv.Itoa(n)
		}
		nodeType := "step"
		for _, sn := range gi.StartingNodes {
			if sn == n {
				nodeType = "step" // starting steps are still steps
				break
			}
		}
		for _, tn := range gi.TerminalNodes {
			if tn == n {
				nodeType = "step"
				break
			}
		}
		nodes = append(nodes, adapter.GraphNode{
			ID:    strconv.Itoa(n),
			Label: label,
			Type:  nodeType,
		})
	}

	var edges []adapter.GraphEdge
	for _, t := range gi.Transitions {
		edges = append(edges, adapter.GraphEdge{
			SourceID: strconv.Itoa(t.From),
			TargetID: strconv.Itoa(t.To),
		})
	}

	return &adapter.GraphData{
		WorkflowID: workflowID,
		Nodes:      nodes,
		Edges:      edges,
	}, nil
}

// --- Traces ---

func (a *Adapter) GetTrace(ctx context.Context, traceID string) (*adapter.TraceData, error) {
	// Query all runs with this trace ID via record store list + metadata filter
	records, err := a.recordStore.List(ctx, "", 0, 100, workflow.OrderTypeAscending)
	if err != nil {
		return nil, err
	}

	var spans []adapter.TraceSpan
	for _, rec := range records {
		if rec.Meta.TraceID != traceID {
			continue
		}
		run := recordToRunData(&rec)
		spans = append(spans, adapter.TraceSpan{
			SpanID:       rec.Meta.SpanID,
			ParentSpanID: rec.Meta.ParentSpanID,
			Run:          run,
			SubsystemID:  rec.Subsystem(),
		})
	}

	if len(spans) == 0 {
		return nil, fmt.Errorf("trace not found: %s", traceID)
	}

	return &adapter.TraceData{
		TraceID: traceID,
		Spans:   spans,
	}, nil
}

// --- Runs ---

func (a *Adapter) ListRuns(ctx context.Context, filter adapter.RunFilter, cursor string, limit int) ([]adapter.RunData, string, error) {
	if limit <= 0 {
		limit = 25
	}

	var offset int64
	if cursor != "" {
		var err error
		offset, err = strconv.ParseInt(cursor, 10, 64)
		if err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
	}

	// Build record filters from the adapter filter.
	var filters []workflow.RecordFilter
	if filter.ForeignID != "" {
		filters = append(filters, workflow.FilterByForeignID(filter.ForeignID))
	}
	if len(filter.Statuses) > 0 {
		var statuses []int
		for _, s := range filter.Statuses {
			rs := runStatusToRunState(s)
			if rs != workflow.RunStateUnknown {
				filters = append(filters, workflow.FilterByRunState(rs))
			}
		}
		_ = statuses
	}

	// Determine workflow name filter. If multiple workflow IDs, use first.
	workflowName := ""
	if len(filter.WorkflowIDs) == 1 {
		workflowName = filter.WorkflowIDs[0]
	}

	records, err := a.recordStore.List(ctx, workflowName, offset, limit, workflow.OrderTypeDescending, filters...)
	if err != nil {
		return nil, "", fmt.Errorf("list runs: %w", err)
	}

	runs := make([]adapter.RunData, 0, len(records))
	for i := range records {
		runs = append(runs, recordToRunData(&records[i]))
	}

	var nextCursor string
	if len(records) == limit {
		nextCursor = strconv.FormatInt(offset+int64(limit), 10)
	}

	return runs, nextCursor, nil
}

func (a *Adapter) GetRun(ctx context.Context, runID string) (*adapter.RunData, error) {
	rec, err := a.recordStore.Lookup(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("get run: %w", err)
	}
	run := recordToRunData(rec)
	return &run, nil
}

func (a *Adapter) GetRunSteps(ctx context.Context, runID string) ([]adapter.StepData, error) {
	if a.stepStore == nil {
		return nil, fmt.Errorf("step store not configured")
	}

	rec, err := a.recordStore.Lookup(ctx, runID)
	if err != nil {
		return nil, fmt.Errorf("lookup run for steps: %w", err)
	}

	steps, err := a.stepStore.List(ctx, rec.WorkflowName, runID)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}

	result := make([]adapter.StepData, 0, len(steps))
	for _, s := range steps {
		result = append(result, stepRecordToStepData(&s))
	}
	return result, nil
}

func (a *Adapter) GetRunTimeline(ctx context.Context, runID string) (*adapter.TimelineData, error) {
	steps, err := a.GetRunSteps(ctx, runID)
	if err != nil {
		return nil, err
	}

	rec, err := a.recordStore.Lookup(ctx, runID)
	if err != nil {
		return nil, err
	}

	var entries []adapter.TimelineEntry
	for _, s := range steps {
		entries = append(entries, adapter.TimelineEntry{
			StepName: s.StepName,
			Status:   s.Status,
			Attempt:  s.Attempt,
			Duration: s.Duration,
		})
	}

	totalDuration := rec.UpdatedAt.Sub(rec.CreatedAt)
	return &adapter.TimelineData{
		RunID:         runID,
		TotalDuration: totalDuration,
		Entries:       entries,
	}, nil
}

func (a *Adapter) GetRunLogs(_ context.Context, _ string, _ string, _ string, _ int) ([]adapter.LogData, string, error) {
	return nil, "", fmt.Errorf("log retrieval not supported by luno/workflow")
}

// --- Actions ---

func (a *Adapter) RetryRun(ctx context.Context, runID string) (string, error) {
	rec, err := a.recordStore.Lookup(ctx, runID)
	if err != nil {
		return "", fmt.Errorf("lookup for retry: %w", err)
	}

	a.mu.RLock()
	entry, ok := a.workflows[rec.WorkflowName]
	a.mu.RUnlock()

	if !ok || entry.reg.RetryFn == nil {
		return "", fmt.Errorf("retry not available for workflow: %s", rec.WorkflowName)
	}

	return entry.reg.RetryFn(ctx, runID)
}

func (a *Adapter) RetryFromStep(_ context.Context, _ string, _ string) (string, error) {
	return "", fmt.Errorf("retry from step not supported by luno/workflow")
}

func (a *Adapter) CancelRun(ctx context.Context, runID string, reason string) error {
	return a.runStateAction(ctx, runID, func(ctrl workflow.RunStateController) error {
		return ctrl.Cancel(ctx, reason)
	})
}

func (a *Adapter) PauseRun(ctx context.Context, runID string) error {
	return a.runStateAction(ctx, runID, func(ctrl workflow.RunStateController) error {
		return ctrl.Pause(ctx, "paused via FlowWatch")
	})
}

func (a *Adapter) ResumeRun(ctx context.Context, runID string) error {
	return a.runStateAction(ctx, runID, func(ctrl workflow.RunStateController) error {
		return ctrl.Resume(ctx)
	})
}

func (a *Adapter) SkipStep(_ context.Context, _ string, _ string, _ string) error {
	return fmt.Errorf("skip step not supported by luno/workflow")
}

func (a *Adapter) runStateAction(ctx context.Context, runID string, action func(workflow.RunStateController) error) error {
	rec, err := a.recordStore.Lookup(ctx, runID)
	if err != nil {
		return fmt.Errorf("lookup for action: %w", err)
	}

	ctrl := workflow.NewRunStateController(a.recordStore.Store, rec)
	return action(ctrl)
}

// --- Streaming (not yet implemented) ---

func (a *Adapter) SubscribeRuns(_ context.Context, _ adapter.RunFilter) (<-chan adapter.RunEvent, error) {
	return nil, fmt.Errorf("streaming not yet implemented")
}

func (a *Adapter) SubscribeRun(_ context.Context, _ string) (<-chan adapter.RunDetailEvent, error) {
	return nil, fmt.Errorf("streaming not yet implemented")
}

// --- Analytics (basic implementations) ---

func (a *Adapter) GetThroughput(_ context.Context, _ adapter.AnalyticsParams) ([]adapter.ThroughputPoint, error) {
	return nil, fmt.Errorf("analytics not yet implemented")
}

func (a *Adapter) GetLatency(_ context.Context, _ adapter.AnalyticsParams) ([]adapter.LatencyPoint, error) {
	return nil, fmt.Errorf("analytics not yet implemented")
}

func (a *Adapter) GetFailureRate(_ context.Context, _ adapter.AnalyticsParams) ([]adapter.FailureRatePoint, error) {
	return nil, fmt.Errorf("analytics not yet implemented")
}

func (a *Adapter) GetStepDuration(_ context.Context, _ adapter.StepDurationParams) (*adapter.StepDurationReport, error) {
	return nil, fmt.Errorf("analytics not yet implemented")
}

func (a *Adapter) GetStepHeatmap(_ context.Context, _ adapter.AnalyticsParams) ([]adapter.HeatmapPoint, error) {
	return nil, fmt.Errorf("analytics not yet implemented")
}

// --- helpers ---

func toWorkflowDef(entry *workflowEntry) adapter.WorkflowDef {
	return adapter.WorkflowDef{
		ID:          entry.reg.Name,
		Name:        entry.reg.Name,
		Engine:      "luno/workflow",
		SubsystemID: entry.reg.Subsystem,
		Metadata:    entry.reg.Metadata,
	}
}
