package server

import (
	"net/http"

	"connectrpc.com/connect"

	"github.com/luno/workflow/flowwatch/internal/adapter"
)

// RegisterAll mounts all FlowWatch service handlers onto the given mux.
// Each Connect-Go handler registers at its canonical path prefix.
func RegisterAll(mux *http.ServeMux, a adapter.EngineAdapter) {
	interceptors := connect.WithInterceptors(
		NewLoggingInterceptor(),
	)

	workflowPath, workflowHandler := newWorkflowServiceHandler(a, interceptors)
	mux.Handle(workflowPath, workflowHandler)

	runPath, runHandler := newRunServiceHandler(a, interceptors)
	mux.Handle(runPath, runHandler)

	searchPath, searchHandler := newSearchServiceHandler(interceptors)
	mux.Handle(searchPath, searchHandler)

	analyticsPath, analyticsHandler := newAnalyticsServiceHandler(a, interceptors)
	mux.Handle(analyticsPath, analyticsHandler)

	streamPath, streamHandler := newStreamServiceHandler(a, interceptors)
	mux.Handle(streamPath, streamHandler)
}
