package lunoworkflow_test

import (
	"context"
	"testing"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memstepstore"

	"github.com/lunarforge/workflow/flowwatch/internal/adapter"
	"github.com/lunarforge/workflow/flowwatch/internal/lunoworkflow"
)

func TestAdapterMetadata(t *testing.T) {
	rs := memrecordstore.New()
	a := lunoworkflow.New(rs, nil)

	if a.EngineName() != "luno/workflow" {
		t.Fatalf("expected engine name 'luno/workflow', got %q", a.EngineName())
	}

	caps := a.Capabilities()
	if !caps.RetryRun {
		t.Fatal("expected RetryRun capability")
	}
	if !caps.PauseRun {
		t.Fatal("expected PauseRun capability")
	}
	if caps.RetryFromStep {
		t.Fatal("expected RetryFromStep to be unsupported")
	}
	if caps.Analytics {
		t.Fatal("expected Analytics false when no step store")
	}

	// With step store
	ss := memstepstore.New()
	a2 := lunoworkflow.New(rs, ss)
	if !a2.Capabilities().Analytics {
		t.Fatal("expected Analytics true with step store")
	}
}

func TestAdapterRegisterAndListWorkflows(t *testing.T) {
	rs := memrecordstore.New()
	a := lunoworkflow.New(rs, nil)

	a.Register(lunoworkflow.WorkflowRegistration{
		Name:      "order-processing",
		Subsystem: "orders",
		Labels:    map[int]string{1: "Created", 2: "Processing", 3: "Completed"},
		GraphInfo: workflow.GraphInfo{
			Nodes:         []int{1, 2, 3},
			StartingNodes: []int{1},
			TerminalNodes: []int{3},
			Transitions:   []workflow.GraphTransition{{From: 1, To: 2}, {From: 2, To: 3}},
		},
	})

	ctx := context.Background()

	workflows, err := a.ListWorkflows(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(workflows))
	}
	if workflows[0].Name != "order-processing" {
		t.Fatalf("expected 'order-processing', got %q", workflows[0].Name)
	}
	if workflows[0].SubsystemID != "orders" {
		t.Fatalf("expected subsystem 'orders', got %q", workflows[0].SubsystemID)
	}
}

func TestAdapterGetGraph(t *testing.T) {
	rs := memrecordstore.New()
	a := lunoworkflow.New(rs, nil)

	a.Register(lunoworkflow.WorkflowRegistration{
		Name:   "test-wf",
		Labels: map[int]string{1: "Start", 2: "Middle", 3: "End"},
		GraphInfo: workflow.GraphInfo{
			Nodes:         []int{1, 2, 3},
			StartingNodes: []int{1},
			TerminalNodes: []int{3},
			Transitions:   []workflow.GraphTransition{{From: 1, To: 2}, {From: 2, To: 3}},
		},
	})

	ctx := context.Background()
	graph, err := a.GetGraph(ctx, "test-wf")
	if err != nil {
		t.Fatal(err)
	}

	if len(graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(graph.Nodes))
	}
	if len(graph.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(graph.Edges))
	}
	if graph.Nodes[0].Label != "Start" {
		t.Fatalf("expected label 'Start', got %q", graph.Nodes[0].Label)
	}
}

func TestAdapterListRuns(t *testing.T) {
	rs := memrecordstore.New()
	a := lunoworkflow.New(rs, nil)

	ctx := context.Background()

	// Store a record directly
	rec := &workflow.Record{
		WorkflowName: "test-wf",
		ForeignID:    "user-123",
		RunID:        "run-1",
		RunState:     workflow.RunStateRunning,
		Status:       1,
		Object:       []byte("{}"),
		Meta: workflow.Meta{
			StatusDescription: "Processing",
			Version:           1,
		},
	}
	err := rs.Store(ctx, rec)
	if err != nil {
		t.Fatal(err)
	}

	runs, cursor, err := a.ListRuns(ctx, adapter.RunFilter{}, "", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	if runs[0].ID != "run-1" {
		t.Fatalf("expected run ID 'run-1', got %q", runs[0].ID)
	}
	if runs[0].Status != "running" {
		t.Fatalf("expected status 'running', got %q", runs[0].Status)
	}
	_ = cursor
}

func TestAdapterGetRun(t *testing.T) {
	rs := memrecordstore.New()
	a := lunoworkflow.New(rs, nil)

	ctx := context.Background()

	rec := &workflow.Record{
		WorkflowName: "test-wf",
		ForeignID:    "user-456",
		RunID:        "run-2",
		RunState:     workflow.RunStateCompleted,
		Status:       3,
		Object:       []byte("{}"),
		Meta: workflow.Meta{
			StatusDescription: "Completed",
			Version:           2,
			TraceID:           "trace-abc",
		},
		Metadata: map[string]string{"env": "prod"},
	}
	err := rs.Store(ctx, rec)
	if err != nil {
		t.Fatal(err)
	}

	run, err := a.GetRun(ctx, "run-2")
	if err != nil {
		t.Fatal(err)
	}

	if run.Status != "succeeded" {
		t.Fatalf("expected 'succeeded', got %q", run.Status)
	}
	if run.Trace.TraceID != "trace-abc" {
		t.Fatalf("expected trace ID 'trace-abc', got %q", run.Trace.TraceID)
	}
	if run.Metadata["env"] != "prod" {
		t.Fatalf("expected metadata env=prod")
	}
	if run.FinishedAt == nil {
		t.Fatal("expected FinishedAt to be set for completed run")
	}
}

func TestAdapterSubsystems(t *testing.T) {
	rs := memrecordstore.New()
	a := lunoworkflow.New(rs, nil)

	a.Register(lunoworkflow.WorkflowRegistration{Name: "wf1", Subsystem: "billing"})
	a.Register(lunoworkflow.WorkflowRegistration{Name: "wf2", Subsystem: "billing"})
	a.Register(lunoworkflow.WorkflowRegistration{Name: "wf3", Subsystem: "orders"})

	ctx := context.Background()

	subs, err := a.ListSubsystems(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if len(subs) != 2 {
		t.Fatalf("expected 2 subsystems, got %d", len(subs))
	}

	detail, err := a.GetSubsystem(ctx, "billing")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Summary.WorkflowCount != 2 {
		t.Fatalf("expected 2 workflows in billing, got %d", detail.Summary.WorkflowCount)
	}
	if len(detail.Workflows) != 2 {
		t.Fatalf("expected 2 workflow defs, got %d", len(detail.Workflows))
	}
}
