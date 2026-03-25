package server

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/lunarforge/workflow/flowwatch/gen/flowwatch/v1/flowwatchv1connect"
	"github.com/lunarforge/workflow/flowwatch/internal/adapter"
)

// workflowServer implements WorkflowServiceHandler by delegating to the EngineAdapter.
type workflowServer struct {
	flowwatchv1connect.UnimplementedWorkflowServiceHandler
	adapter adapter.EngineAdapter
}

// runServer implements RunServiceHandler by delegating to the EngineAdapter.
type runServer struct {
	flowwatchv1connect.UnimplementedRunServiceHandler
	adapter adapter.EngineAdapter
}

// searchServer implements SearchServiceHandler.
type searchServer struct {
	flowwatchv1connect.UnimplementedSearchServiceHandler
}

// analyticsServer implements AnalyticsServiceHandler by delegating to the EngineAdapter.
type analyticsServer struct {
	flowwatchv1connect.UnimplementedAnalyticsServiceHandler
	adapter adapter.EngineAdapter
}

// streamServer implements StreamServiceHandler by delegating to the EngineAdapter.
type streamServer struct {
	flowwatchv1connect.UnimplementedStreamServiceHandler
	adapter adapter.EngineAdapter
}

func newWorkflowServiceHandler(a adapter.EngineAdapter, opts ...connect.HandlerOption) (string, http.Handler) {
	return flowwatchv1connect.NewWorkflowServiceHandler(&workflowServer{adapter: a}, opts...)
}

func newRunServiceHandler(a adapter.EngineAdapter, opts ...connect.HandlerOption) (string, http.Handler) {
	return flowwatchv1connect.NewRunServiceHandler(&runServer{adapter: a}, opts...)
}

func newSearchServiceHandler(opts ...connect.HandlerOption) (string, http.Handler) {
	return flowwatchv1connect.NewSearchServiceHandler(&searchServer{}, opts...)
}

func newAnalyticsServiceHandler(a adapter.EngineAdapter, opts ...connect.HandlerOption) (string, http.Handler) {
	return flowwatchv1connect.NewAnalyticsServiceHandler(&analyticsServer{adapter: a}, opts...)
}

func newStreamServiceHandler(a adapter.EngineAdapter, opts ...connect.HandlerOption) (string, http.Handler) {
	return flowwatchv1connect.NewStreamServiceHandler(&streamServer{adapter: a}, opts...)
}
