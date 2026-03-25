// Package adapter defines the engine-agnostic interface that workflow engines
// must implement to integrate with FlowWatch.
//
// Each workflow engine (luno/workflow, Temporal, etc.) provides a concrete
// implementation of EngineAdapter. The FlowWatch server delegates all
// engine-specific operations through this interface.
package adapter

import (
	"context"
	"time"
)

// ─── Core adapter interface ───

// EngineAdapter abstracts workflow engine operations behind a unified interface.
type EngineAdapter interface {
	// Metadata
	EngineName() string
	Capabilities() *EngineCapabilities

	// Subsystems
	ListSubsystems(ctx context.Context) ([]SubsystemSummary, error)
	GetSubsystem(ctx context.Context, subsystemID string) (*SubsystemDetail, error)

	// Workflow definitions
	ListWorkflows(ctx context.Context) ([]WorkflowDef, error)
	GetWorkflow(ctx context.Context, workflowID string) (*WorkflowDef, error)
	GetGraph(ctx context.Context, workflowID string) (*GraphData, error)

	// Traces
	GetTrace(ctx context.Context, traceID string) (*TraceData, error)

	// Runs
	ListRuns(ctx context.Context, filter RunFilter, cursor string, limit int) ([]RunData, string, error)
	GetRun(ctx context.Context, runID string) (*RunData, error)
	GetRunSteps(ctx context.Context, runID string) ([]StepData, error)
	GetRunTimeline(ctx context.Context, runID string) (*TimelineData, error)
	GetRunLogs(ctx context.Context, runID string, stepName string, cursor string, limit int) ([]LogData, string, error)

	// Actions
	RetryRun(ctx context.Context, runID string) (newRunID string, err error)
	RetryFromStep(ctx context.Context, runID, stepName string) (newRunID string, err error)
	CancelRun(ctx context.Context, runID string, reason string) error
	PauseRun(ctx context.Context, runID string) error
	ResumeRun(ctx context.Context, runID string) error
	SkipStep(ctx context.Context, runID, stepName string, reason string) error

	// Streaming
	SubscribeRuns(ctx context.Context, filter RunFilter) (<-chan RunEvent, error)
	SubscribeRun(ctx context.Context, runID string) (<-chan RunDetailEvent, error)

	// Analytics
	GetThroughput(ctx context.Context, p AnalyticsParams) ([]ThroughputPoint, error)
	GetLatency(ctx context.Context, p AnalyticsParams) ([]LatencyPoint, error)
	GetFailureRate(ctx context.Context, p AnalyticsParams) ([]FailureRatePoint, error)
	GetStepDuration(ctx context.Context, p StepDurationParams) (*StepDurationReport, error)
	GetStepHeatmap(ctx context.Context, p AnalyticsParams) ([]HeatmapPoint, error)
}

// ─── Capability negotiation ───

type EngineCapabilities struct {
	RetryRun      bool
	RetryFromStep bool
	CancelRun     bool
	PauseRun      bool
	ResumeRun     bool
	SkipStep      bool
	Streaming     bool
	Analytics     bool
}

// ─── Domain types (engine-agnostic) ───
// These are mapped to/from proto messages in the server layer.

type WorkflowDef struct {
	ID          string
	Name        string
	Engine      string
	Version     string
	SubsystemID string
	CreatedAt   time.Time
	Metadata    map[string]string
}

// ─── Subsystem types ───

type SubsystemSummary struct {
	Subsystem       SubsystemData
	WorkflowCount   int
	ActiveRuns      int
	FailedLastHour  int
	ErrorRateLastHr float64
}

type SubsystemDetail struct {
	Summary   SubsystemSummary
	Workflows []WorkflowDef
}

type SubsystemData struct {
	ID          string
	Name        string
	Description string
	Metadata    map[string]string
	CreatedAt   time.Time
}

// ─── Trace types ───

type TraceData struct {
	TraceID        string
	Spans          []TraceSpan
	TotalDuration  time.Duration
	SubsystemCount int
}

type TraceSpan struct {
	SpanID       string
	ParentSpanID string
	Run          RunData
	SubsystemID  string
	Offset       time.Duration
	Duration     time.Duration
	Steps        []StepData
}

// ─── Trace context (attached to runs) ───

type TraceContextData struct {
	TraceID            string
	SpanID             string
	ParentSpanID       string
	AdditionalTraceIDs []string
	TraceSource        string // http-header, kafka-header, grpc-metadata, manual
}

type GraphData struct {
	WorkflowID string
	Nodes      []GraphNode
	Edges      []GraphEdge
}

type GraphNode struct {
	ID       string
	Label    string
	Type     string // step, condition, fork, join, subworkflow
	Metadata map[string]string
}

type GraphEdge struct {
	SourceID string
	TargetID string
	Label    string
}

type RunData struct {
	ID          string
	WorkflowID  string
	ForeignID   string
	SubsystemID string
	Status      string
	CurrentStep string
	Trace       TraceContextData
	StartedAt   time.Time
	FinishedAt  *time.Time
	Duration    time.Duration
	Error       *ErrorData
	Metadata    map[string]string
}

type StepData struct {
	ID         string
	RunID      string
	StepName   string
	Status     string
	Attempt    int
	StartedAt  time.Time
	FinishedAt *time.Time
	Duration   time.Duration
	QueueWait  time.Duration
	Input      map[string]any
	Output     map[string]any
	Error      *ErrorData
}

type ErrorData struct {
	Message    string
	Code       string
	StackTrace string
}

type LogData struct {
	RunID      string
	StepName   string
	LineNumber int64
	Timestamp  time.Time
	Level      string
	Message    string
	Source     string
}

type TimelineData struct {
	RunID         string
	TotalDuration time.Duration
	Entries       []TimelineEntry
}

type TimelineEntry struct {
	StepName  string
	Status    string
	Attempt   int
	Offset    time.Duration
	Duration  time.Duration
	QueueWait time.Duration
	Error     *ErrorData
}

// ─── Filters ───

type RunFilter struct {
	WorkflowIDs   []string
	SubsystemIDs  []string
	Statuses      []string
	ForeignID     string
	TraceID       string
	From          *time.Time
	To            *time.Time
	MetadataMatch map[string]string
}

// ─── Events ───

type RunEvent struct {
	Type string // created, updated, completed
	Run  RunData
}

type RunDetailEvent struct {
	Type string // run_updated, step_updated, log_appended
	Run  *RunData
	Step *StepData
	Log  *LogData
}

// ─── Analytics params ───

type AnalyticsParams struct {
	WorkflowID  string
	From        time.Time
	To          time.Time
	Granularity string // 1m, 5m, 15m, 1h, 1d
}

type StepDurationParams struct {
	WorkflowID string // empty = all
	From       time.Time
	To         time.Time
}

type ThroughputPoint struct {
	Timestamp time.Time
	Succeeded int
	Failed    int
	Running   int
	Canceled  int
	Total     int
}

type LatencyPoint struct {
	Timestamp time.Time
	P50       time.Duration
	P95       time.Duration
	P99       time.Duration
}

type FailureRatePoint struct {
	Timestamp  time.Time
	Rate       float64
	TotalRuns  int
	FailedRuns int
}

type StepDurationReport struct {
	Steps                []StepDurationEntry
	BottleneckThreshold  time.Duration
}

type StepDurationEntry struct {
	StepName     string
	IsBottleneck bool
	QueueWait    DurationPercentiles
	Execution    DurationPercentiles
	Total        DurationPercentiles
	SampleCount  int64
	Trend        []StepTrendPoint
}

type DurationPercentiles struct {
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
}

type StepTrendPoint struct {
	Timestamp time.Time
	P50       time.Duration
	P95       time.Duration
	P99       time.Duration
}

type HeatmapPoint struct {
	StepName        string
	BucketStart     time.Time
	FailureRate     float64
	TotalExecutions int
	FailedExecs     int
	AvgDuration     time.Duration
}
