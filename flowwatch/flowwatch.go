// Package flowwatch provides the public API for integrating FlowWatch with
// luno/workflow applications. It re-exports the adapter and server wiring
// needed to add monitoring to any workflow application.
//
// Quickstart:
//
//	adapter := flowwatch.NewAdapter(recordStore, nil)
//	flowwatch.RegisterWorkflow(adapter, wf)
//
//	mux := http.NewServeMux()
//	flowwatch.RegisterHandlers(mux, adapter)
package flowwatch

import (
	"net/http"

	"github.com/luno/workflow"

	"github.com/luno/workflow/flowwatch/internal/lunoworkflow"
	"github.com/luno/workflow/flowwatch/internal/server"
)

// Adapter is a FlowWatch engine adapter for luno/workflow.
type Adapter = lunoworkflow.Adapter

// NewAdapter creates a new FlowWatch adapter backed by the given record store
// and optional step store. Pass nil for stepStore if step-level tracking is
// not needed.
func NewAdapter(recordStore workflow.RecordStore, stepStore workflow.StepStore) *Adapter {
	return lunoworkflow.New(recordStore, stepStore)
}

// RegisterOption configures optional fields during workflow registration.
type RegisterOption func(*Registration)

// WithSubsystem assigns the workflow to a subsystem for grouped monitoring.
// If not set, the workflow defaults to the "default" subsystem.
func WithSubsystem(subsystem string) RegisterOption {
	return func(r *Registration) { r.Subsystem = subsystem }
}

// RegisterWorkflow extracts graph topology and status labels from a typed
// Workflow instance and registers it with the adapter for monitoring.
func RegisterWorkflow[Type any, Status workflow.StatusType](a *Adapter, wf *workflow.Workflow[Type, Status], opts ...RegisterOption) {
	fns := make([]func(*Registration), len(opts))
	for i, opt := range opts {
		fns[i] = func(r *Registration) { opt(r) }
	}
	lunoworkflow.RegisterWorkflow(a, wf, fns...)
}

// Registration exposes WorkflowRegistration for manual registration without
// a live Workflow instance.
type Registration = lunoworkflow.WorkflowRegistration

// RegisterHandlers mounts all FlowWatch Connect-Go service handlers onto the
// given mux. The adapter provides the engine-specific backend.
func RegisterHandlers(mux *http.ServeMux, a *Adapter) {
	server.RegisterAll(mux, a)
}

// RetryFunc is a function that retries a finished workflow run.
type RetryFunc = lunoworkflow.RetryFunc

// --- Convenience types re-exported for manual registration ---

// Register adds a workflow to the adapter using a manual Registration struct.
// Use this when you don't have a live *workflow.Workflow instance (e.g., in tests).
func Register(a *Adapter, reg Registration) {
	a.Register(reg)
}

// --- Health check handler ---

// HealthHandler returns an http.HandlerFunc that responds with "ok" on 200.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	}
}

