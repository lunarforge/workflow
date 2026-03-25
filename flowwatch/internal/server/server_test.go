package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"connectrpc.com/connect"

	flowwatchv1 "github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1"
	"github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1/flowwatchv1connect"
	"github.com/lunarforge/workflow/flowwatch/internal/lunoworkflow"
	"github.com/lunarforge/workflow/flowwatch/internal/server"

	"github.com/lunarforge/workflow"
	"github.com/lunarforge/workflow/adapters/memrecordstore"
	"github.com/lunarforge/workflow/adapters/memstepstore"
)

func setup(t *testing.T) (*httptest.Server, *lunoworkflow.Adapter, *memrecordstore.Store) {
	t.Helper()

	rs := memrecordstore.New()
	ss := memstepstore.New()
	a := lunoworkflow.New(rs, ss)

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

	mux := http.NewServeMux()
	server.RegisterAll(mux, a)

	ts := httptest.NewUnstartedServer(mux)
	ts.EnableHTTP2 = true
	ts.StartTLS()
	t.Cleanup(ts.Close)

	return ts, a, rs
}

func TestListWorkflows(t *testing.T) {
	ts, _, _ := setup(t)

	client := flowwatchv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	res, err := client.ListWorkflows(context.Background(), connect.NewRequest(&flowwatchv1.ListWorkflowsRequest{}))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Msg.Workflows) != 1 {
		t.Fatalf("expected 1 workflow, got %d", len(res.Msg.Workflows))
	}
	if res.Msg.Workflows[0].Name != "order-processing" {
		t.Fatalf("expected 'order-processing', got %q", res.Msg.Workflows[0].Name)
	}
}

func TestGetGraph(t *testing.T) {
	ts, _, _ := setup(t)

	client := flowwatchv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	res, err := client.GetGraph(context.Background(), connect.NewRequest(&flowwatchv1.GetGraphRequest{
		WorkflowId: "order-processing",
	}))
	if err != nil {
		t.Fatal(err)
	}

	if res.Msg.Graph == nil {
		t.Fatal("expected graph")
	}
	if len(res.Msg.Graph.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(res.Msg.Graph.Nodes))
	}
	if len(res.Msg.Graph.Edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(res.Msg.Graph.Edges))
	}
}

func TestGetCapabilities(t *testing.T) {
	ts, _, _ := setup(t)

	client := flowwatchv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	res, err := client.GetCapabilities(context.Background(), connect.NewRequest(&flowwatchv1.GetCapabilitiesRequest{}))
	if err != nil {
		t.Fatal(err)
	}

	if res.Msg.Capabilities == nil {
		t.Fatal("expected capabilities")
	}
	if !res.Msg.Capabilities.RetryRun {
		t.Fatal("expected RetryRun capability")
	}
	if !res.Msg.Capabilities.PauseRun {
		t.Fatal("expected PauseRun capability")
	}
	if !res.Msg.Capabilities.Analytics {
		t.Fatal("expected Analytics capability with step store")
	}
}

func TestListAndGetRun(t *testing.T) {
	ts, _, rs := setup(t)

	ctx := context.Background()

	rec := &workflow.Record{
		WorkflowName: "order-processing",
		ForeignID:    "order-999",
		RunID:        "run-abc",
		RunState:     workflow.RunStateRunning,
		Status:       2,
		Object:       []byte("{}"),
		Meta: workflow.Meta{
			StatusDescription: "Processing",
			Version:           1,
		},
	}
	if err := rs.Store(ctx, rec); err != nil {
		t.Fatal(err)
	}

	runClient := flowwatchv1connect.NewRunServiceClient(ts.Client(), ts.URL)

	// ListRuns
	listRes, err := runClient.ListRuns(ctx, connect.NewRequest(&flowwatchv1.ListRunsRequest{}))
	if err != nil {
		t.Fatal(err)
	}
	if len(listRes.Msg.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(listRes.Msg.Runs))
	}
	if listRes.Msg.Runs[0].Id != "run-abc" {
		t.Fatalf("expected run ID 'run-abc', got %q", listRes.Msg.Runs[0].Id)
	}

	// GetRun
	getRes, err := runClient.GetRun(ctx, connect.NewRequest(&flowwatchv1.GetRunRequest{RunId: "run-abc"}))
	if err != nil {
		t.Fatal(err)
	}
	if getRes.Msg.Run.ForeignId != "order-999" {
		t.Fatalf("expected foreign ID 'order-999', got %q", getRes.Msg.Run.ForeignId)
	}
	if getRes.Msg.Run.Status != flowwatchv1.RunStatus_RUN_STATUS_RUNNING {
		t.Fatalf("expected RUN_STATUS_RUNNING, got %v", getRes.Msg.Run.Status)
	}
}

func TestListSubsystems(t *testing.T) {
	ts, _, _ := setup(t)

	client := flowwatchv1connect.NewWorkflowServiceClient(ts.Client(), ts.URL)
	res, err := client.ListSubsystems(context.Background(), connect.NewRequest(&flowwatchv1.ListSubsystemsRequest{}))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Msg.Subsystems) != 1 {
		t.Fatalf("expected 1 subsystem, got %d", len(res.Msg.Subsystems))
	}
	if res.Msg.Subsystems[0].Subsystem == nil || res.Msg.Subsystems[0].Subsystem.Id != "orders" {
		t.Fatalf("expected subsystem 'orders', got %v", res.Msg.Subsystems[0].Subsystem)
	}
}

func TestRunFilterByWorkflow(t *testing.T) {
	ts, _, rs := setup(t)
	ctx := context.Background()

	for _, r := range []*workflow.Record{
		{WorkflowName: "order-processing", RunID: "run-1", RunState: workflow.RunStateRunning, Status: 1, Object: []byte("{}"), Meta: workflow.Meta{Version: 1}},
		{WorkflowName: "other-workflow", RunID: "run-2", RunState: workflow.RunStateRunning, Status: 1, Object: []byte("{}"), Meta: workflow.Meta{Version: 1}},
	} {
		if err := rs.Store(ctx, r); err != nil {
			t.Fatal(err)
		}
	}

	runClient := flowwatchv1connect.NewRunServiceClient(ts.Client(), ts.URL)
	res, err := runClient.ListRuns(ctx, connect.NewRequest(&flowwatchv1.ListRunsRequest{
		Filter: &flowwatchv1.RunFilter{
			WorkflowIds: []string{"order-processing"},
		},
	}))
	if err != nil {
		t.Fatal(err)
	}

	if len(res.Msg.Runs) != 1 {
		t.Fatalf("expected 1 filtered run, got %d", len(res.Msg.Runs))
	}
	if res.Msg.Runs[0].WorkflowId != "order-processing" {
		t.Fatalf("expected 'order-processing', got %q", res.Msg.Runs[0].WorkflowId)
	}
}
