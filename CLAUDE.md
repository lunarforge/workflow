# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`github.com/lunarforge/workflow` is a type-safe, event-driven workflow orchestration library for Go. It uses Go generics to provide compile-time safety for building distributed state machines. Workflows are parameterized by two types: a data type (`Type`) and a status enum (`Status`).

## Build & Test Commands

```bash
# Run all core tests
go test -v -race -timeout=5m ./...

# Run a single test
go test -v -race -run TestName ./...

# Vet
go vet -v ./...

# Test a specific adapter (each adapter is a separate Go module)
cd adapters/memrecordstore && go test ./...

# Integration tests (require Docker for testcontainers)
cd adapters/kafkastreamer && go test -v -timeout=10m ./...
cd adapters/wredis && go test -v -timeout=10m ./...
```

## Architecture

### Core Pattern: Builder -> Workflow -> Run

1. **Builder** (`builder.go`) - Configures a workflow using `NewBuilder[Type, Status]()`. Registers steps, callbacks, timeouts, and connectors. Call `Build()` with infrastructure adapters to get a `Workflow`.

2. **Workflow** (`workflow.go`) - The runtime engine. `Run(ctx)` starts background goroutines for each consumer (steps, timeouts, connectors, outbox, hooks, delete). `Stop()` gracefully shuts them down. Implements the `API[Type, Status]` interface.

3. **Run** (`run.go`) - Represents a single workflow execution. Passed to step functions with typed access to the object via `r.Object`. Provides `RunStateController` methods (Pause, Cancel, Resume, DeleteData, SaveAndRepeat).

### Three Core Adapter Interfaces

All adapters live in `adapters/` as **separate Go modules** with their own `go.mod`:

- **EventStreamer** (`eventstreamer.go`) - Sends/receives events (e.g., memstreamer, kafkastreamer, reflexstreamer, wredis)
- **RecordStore** (`store.go`) - Persists workflow records with transactional outbox support (e.g., memrecordstore, sqlstore, wredis)
- **RoleScheduler** (`rolescheduler.go`) - Distributed leader election for consumer roles (e.g., memrolescheduler, rinkrolescheduler)
- **TimeoutStore** (`store.go`) - Optional, required only when using timeouts (e.g., memtimeoutstore, sqltimeout)

All adapter implementations should be validated using `adapters/adaptertest/` conformance tests.

### Key Workflow Primitives

- **Steps** (`step.go`, `consumer.go`) - Status-to-status transitions via `ConsumerFunc`. Configured with `AddStep(from, fn, ...destinations)`.
- **Callbacks** (`callback.go`) - External trigger points via `AddCallback(status, fn, ...destinations)`. Invoked via `Callback()` API.
- **Timeouts** (`timeout.go`) - Time-based transitions via `AddTimeout(status, timerFn, timeoutFn, ...destinations)`. Requires a `TimeoutStore`.
- **Connectors** (`connector.go`) - Bridge external event streams into the workflow via `AddConnector()`.
- **Hooks** (`hook.go`) - React to RunState changes (Pause, Cancel, Complete) via `OnPause()`, `OnCancel()`, `OnComplete()`.

### Status Graph

Transitions form a directed graph (`internal/graph/`) validated at build time. The graph ensures workflows have at least one starting point and tracks valid transitions.

### Testing Utilities

`testing.go` provides helpers for integration-testing workflows: `Require()`, `TriggerCallbackOn()`, `AwaitTimeoutInsert()`, `WaitFor()`, `NewTestingRun()`. These require `TestingRecordStore` (implemented by `memrecordstore`).

### Protobuf

Outbox events use protobuf (`internal/outboxpb/`, `workflowpb/`). Object serialization uses `Marshal`/`Unmarshal` functions in `marshal.go`/`unmarshal.go` (JSON by default).

### Test Naming Convention

Tests use two patterns:
- `*_test.go` - External (black-box) tests in `workflow_test` package
- `*_internal_test.go` - Internal (white-box) tests in `workflow` package

## FlowWatch — Monitoring Dashboard

`flowwatch/` is a separate Go module (`github.com/lunarforge/workflow/flowwatch`) providing an engine-agnostic workflow monitoring dashboard with a Connect-Go API and SvelteKit frontend.

### Build & Test

```bash
# Go tests
cd flowwatch && go test -v -race ./...

# Frontend (requires bun)
cd flowwatch/ui && bun install && bun run build

# Regenerate protobuf (requires buf CLI)
cd flowwatch && buf generate    # Go stubs
cd flowwatch/ui && buf generate  # TypeScript stubs
```

### Architecture

- **Proto definitions**: `flowwatch/proto/flowwatch/v1/` — 6 services (Workflow, Run, Search, Analytics, Stream, plus shared types)
- **Generated code**: `flowwatch/gen/` (Go), `flowwatch/ui/src/gen/` (TypeScript)
- **EngineAdapter interface**: `flowwatch/internal/adapter/adapter.go` — abstraction layer between services and workflow engines
- **luno/workflow adapter**: `flowwatch/internal/lunoworkflow/` — concrete adapter using RecordStore and StepStore
- **Connect-Go handlers**: `flowwatch/internal/server/` — service implementations, proto conversion, logging interceptor
- **Server entry point**: `flowwatch/cmd/flowwatch/main.go` — h2c server with CORS support
- **SvelteKit frontend**: `flowwatch/ui/` — Svelte 5 + Tailwind CSS v4 + Connect-Web transport

### Key Design Decisions

- Uses `replace github.com/lunarforge/workflow => ../` in `go.mod` for in-repo development
- `WorkflowRegistration` struct provides type-erased workflow metadata (graph info, labels, retry func) to the adapter
- `RegisterWorkflow[Type, Status]()` generic helper extracts graph info and binds retry at registration time
- RunStateController for Pause/Cancel/Resume actions uses `recordStore.Store` directly
